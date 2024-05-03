package job

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

// Job Instance variables
type Job struct {
	kubeclient                kubernetes.Interface
	radixclient               radixclient.Interface
	kubeutil                  *kube.Kube
	radixJob                  *v1.RadixJob
	originalRadixJobCondition v1.RadixJobCondition
	config                    *apiconfig.Config
	logger                    zerolog.Logger
}

// NewJob Constructor
func NewJob(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, radixJob *v1.RadixJob, config *apiconfig.Config) Job {
	originalRadixJobStatus := radixJob.Status.Condition

	return Job{
		kubeclient:                kubeclient,
		radixclient:               radixclient,
		kubeutil:                  kubeutil,
		radixJob:                  radixJob,
		originalRadixJobCondition: originalRadixJobStatus,
		config:                    config,
		logger:                    log.Logger.With().Str("resource_kind", v1.KindRadixJob).Str("resource_name", cache.MetaObjectToName(&radixJob.ObjectMeta).String()).Logger(),
	}
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (job *Job) OnSync(ctx context.Context) error {
	job.restoreStatus(ctx)

	appName := job.radixJob.Spec.AppName
	ra, err := job.radixclient.RadixV1().RadixApplications(job.radixJob.GetNamespace()).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		job.logger.Debug().Msgf("for BuildDeploy failed to find RadixApplication by name %s", appName)
	}

	if err := job.syncTargetEnvironments(ctx, ra); err != nil {
		return fmt.Errorf("failed to sync target environments: %w", err)
	}

	if IsRadixJobDone(job.radixJob) {
		job.logger.Debug().Msgf("Ignoring RadixJob %s/%s as it's no longer active.", job.radixJob.Namespace, job.radixJob.Name)
		return nil
	}

	stopReconciliation, err := job.syncStatuses(ctx, ra)
	if err != nil {
		return err
	}
	if stopReconciliation {
		job.logger.Info().Msgf("stop reconciliation, status updated triggering new sync")
		return nil
	}

	_, err = job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(ctx, job.radixJob.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		err = job.createPipelineJob(ctx)
		if err != nil {
			return err
		}
		err = job.setStatusOfJob(ctx)
		if err != nil {
			return err
		}
	}

	job.maintainHistoryLimit(ctx)
	job.garbageCollectConfigMaps(ctx)

	return nil
}

// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
func (job *Job) restoreStatus(ctx context.Context) {
	if restoredStatus, ok := job.radixJob.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if job.radixJob.Status.Condition == "" {
			var status v1.RadixJobStatus
			err := json.Unmarshal([]byte(restoredStatus), &status)
			if err != nil {
				job.logger.Error().Err(err).Msg("Unable to get status from annotation")
				return
			}
			err = job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
				currStatus.Condition = status.Condition
				currStatus.Created = status.Created
				currStatus.Started = status.Started
				currStatus.Ended = status.Ended
				currStatus.TargetEnvs = status.TargetEnvs
				currStatus.Steps = status.Steps
			})
			if err != nil {
				job.logger.Error().Err(err).Msg("Unable to restore status")
				return
			}
		}
	}
}

func (job *Job) syncStatuses(ctx context.Context, ra *v1.RadixApplication) (stopReconciliation bool, err error) {
	stopReconciliation = false

	if job.radixJob.Spec.Stop {
		err = job.stopJob(ctx)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	allJobs, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("failed to get all RadixJobs: %v", err)
		return false, err
	}

	if job.isOtherJobRunningOnBranchOrEnvironment(ra, allJobs.Items) {
		err = job.queueJob(ctx)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	err = job.setStatusOfJob(ctx)
	if err != nil {
		return false, err
	}

	return
}

func (job *Job) isOtherJobRunningOnBranchOrEnvironment(ra *v1.RadixApplication, allJobs []v1.RadixJob) bool {
	isJobActive := func(rj *v1.RadixJob) bool {
		return rj.Status.Condition == v1.JobWaiting || rj.Status.Condition == v1.JobRunning
	}

	jobTargetEnvironments := getTargetEnvironments(ra, job)
	for _, rj := range allJobs {
		if rj.GetName() == job.radixJob.GetName() || !isJobActive(&rj) {
			continue
		}
		switch rj.Spec.PipeLineType {
		case v1.BuildDeploy, v1.Build:
			if len(jobTargetEnvironments) > 0 {
				rjTargetBranches := applicationconfig.GetTargetEnvironments(rj.Spec.Build.Branch, ra)
				for _, rjEnvName := range rjTargetBranches {
					if _, ok := jobTargetEnvironments[rjEnvName]; ok {
						return true
					}
				}
			} else if job.radixJob.Spec.Build.Branch == rj.Spec.Build.Branch {
				return true
			}
		case v1.Deploy:
			if _, ok := jobTargetEnvironments[rj.Spec.Deploy.ToEnvironment]; ok {
				return true
			}
		case v1.Promote:
			if _, ok := jobTargetEnvironments[rj.Spec.Promote.ToEnvironment]; ok {
				return true
			}
		}
	}
	return false
}

func getTargetEnvironments(ra *v1.RadixApplication, job *Job) map[string]struct{} {
	targetEnvs := make(map[string]struct{})
	if job.radixJob.Spec.PipeLineType == v1.BuildDeploy || job.radixJob.Spec.PipeLineType == v1.Build {
		if ra != nil {
			return slice.Reduce(applicationconfig.GetTargetEnvironments(job.radixJob.Spec.Build.Branch, ra),
				targetEnvs, func(acc map[string]struct{}, envName string) map[string]struct{} {
					acc[envName] = struct{}{}
					return acc
				})
		}
	} else if job.radixJob.Spec.PipeLineType == v1.Deploy {
		targetEnvs[job.radixJob.Spec.Deploy.ToEnvironment] = struct{}{}
	} else if job.radixJob.Spec.PipeLineType == v1.Promote {
		targetEnvs[job.radixJob.Spec.Promote.ToEnvironment] = struct{}{}
	}
	return targetEnvs
}

// sync the environments in the RadixJob with environments in the RA
func (job *Job) syncTargetEnvironments(ctx context.Context, ra *v1.RadixApplication) error {
	rj := job.radixJob

	// TargetEnv has already been set
	if len(rj.Status.TargetEnvs) > 0 {
		return nil
	}
	targetEnvs := job.getTargetEnv(ra, rj)

	// Update RJ with accurate env data
	return job.updateRadixJobStatus(ctx, rj, func(currStatus *v1.RadixJobStatus) {
		if rj.Spec.PipeLineType == v1.Deploy {
			currStatus.TargetEnvs = append(currStatus.TargetEnvs, rj.Spec.Deploy.ToEnvironment)
		} else if rj.Spec.PipeLineType == v1.Promote {
			currStatus.TargetEnvs = append(currStatus.TargetEnvs, rj.Spec.Promote.ToEnvironment)
		} else if rj.Spec.PipeLineType == v1.BuildDeploy {
			currStatus.TargetEnvs = targetEnvs
		}
	})
}

func (job *Job) getTargetEnv(ra *v1.RadixApplication, rj *v1.RadixJob) (targetEnvs []string) {
	if rj.Spec.PipeLineType != v1.BuildDeploy || len(rj.Spec.Build.Branch) == 0 || ra == nil {
		return
	}

	for _, env := range ra.Spec.Environments {
		if len(env.Build.From) > 0 && branch.MatchesPattern(env.Build.From, rj.Spec.Build.Branch) {
			targetEnvs = append(targetEnvs, env.Name)
		}
	}
	return targetEnvs
}

// IsRadixJobDone Checks if job is done
func IsRadixJobDone(rj *v1.RadixJob) bool {
	return rj == nil || isJobConditionDone(rj.Status.Condition)
}

func (job *Job) setStatusOfJob(ctx context.Context) error {
	pipelineJob, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(ctx, job.radixJob.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// No kubernetes job created yet, so nothing to sync
			return nil
		}
		return err
	}

	steps, err := job.getJobSteps(ctx, pipelineJob)
	if err != nil {
		return err
	}
	environments, err := job.getJobEnvironments(ctx)
	if err != nil {
		return err
	}
	jobStatusCondition, err := job.getJobConditionFromJobStatus(ctx, pipelineJob.Status)
	if err != nil {
		return err
	}

	err = job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
		currStatus.Steps = steps
		currStatus.Condition = jobStatusCondition
		currStatus.Created = &job.radixJob.CreationTimestamp
		currStatus.Started = pipelineJob.Status.StartTime
		if len(pipelineJob.Status.Conditions) > 0 {
			currStatus.Ended = &pipelineJob.Status.Conditions[0].LastTransitionTime
		}
		if len(environments) > 0 {
			currStatus.TargetEnvs = environments
		}
	})
	if err != nil {
		return err
	}
	if isJobConditionDone(jobStatusCondition) {
		err = job.setNextJobToRunning(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func isJobConditionDone(jobStatusCondition v1.RadixJobCondition) bool {
	return jobStatusCondition == v1.JobSucceeded || jobStatusCondition == v1.JobFailed ||
		jobStatusCondition == v1.JobStopped || jobStatusCondition == v1.JobStoppedNoChanges
}

func (job *Job) stopJob(ctx context.Context) error {
	isRunning := job.radixJob.Status.Condition == v1.JobRunning
	stoppedSteps, err := job.getStoppedSteps(ctx, isRunning)
	if err != nil {
		return err
	}

	err = job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
		if isRunning && stoppedSteps != nil {
			currStatus.Steps = *stoppedSteps
		}
		currStatus.Created = &job.radixJob.CreationTimestamp
		currStatus.Condition = v1.JobStopped
		currStatus.Ended = &metav1.Time{Time: time.Now()}
	})
	if err != nil {
		return err
	}

	return job.setNextJobToRunning(ctx)
}

func (job *Job) getStoppedSteps(ctx context.Context, isRunning bool) (*[]v1.RadixJobStep, error) {
	if !isRunning {
		return nil, nil
	}
	// Delete pipeline job
	err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Delete(ctx, job.radixJob.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = deleteJobPodIfExistsAndNotCompleted(ctx, job.kubeclient, job.radixJob.Namespace, job.radixJob.Name)
	if err != nil {
		return nil, err
	}

	err = job.deleteStepJobs(ctx)
	if err != nil {
		return nil, err
	}

	stoppedSteps := make([]v1.RadixJobStep, 0)
	for _, step := range job.radixJob.Status.Steps {
		if step.Condition != v1.JobSucceeded && step.Condition != v1.JobFailed {
			step.Condition = v1.JobStopped
		}
		stoppedSteps = append(stoppedSteps, step)
	}
	return &stoppedSteps, nil
}

func (job *Job) deleteStepJobs(ctx context.Context) error {
	jobs, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s!=%s", kube.RadixJobNameLabel, job.radixJob.Name, kube.RadixJobTypeLabel, kube.RadixJobTypeJob),
	})

	if err != nil {
		return err
	}

	if len(jobs.Items) > 0 {
		for _, kubernetesJob := range jobs.Items {
			if kubernetesJob.Status.CompletionTime != nil {
				continue
			}

			err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Delete(ctx, kubernetesJob.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}

			err = deleteJobPodIfExistsAndNotCompleted(ctx, job.kubeclient, job.radixJob.Namespace, kubernetesJob.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (job *Job) setNextJobToRunning(ctx context.Context) error {
	rjList, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.GetNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	rjs := sortRadixJobsByCreatedAsc(rjList.Items)
	for _, otherRj := range rjs {
		if otherRj.Name != job.radixJob.Name && otherRj.Status.Condition == v1.JobQueued { // previous status for this otherRj was Queued
			return job.updateRadixJobStatusWithMetrics(ctx, &otherRj, v1.JobQueued, func(currStatus *v1.RadixJobStatus) {
				currStatus.Condition = v1.JobRunning
			})
		}
	}
	return nil
}

func (job *Job) queueJob(ctx context.Context) error {
	return job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
		currStatus.Created = &job.radixJob.CreationTimestamp
		currStatus.Condition = v1.JobQueued
	})
}

func (job *Job) getJobSteps(ctx context.Context, pipelineJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	var steps []v1.RadixJobStep

	pipelinePod, err := job.getPipelinePod(ctx)
	if err != nil {
		return nil, err
	} else if pipelinePod == nil {
		return steps, nil
	}

	if len(pipelinePod.Status.ContainerStatuses) == 0 {
		return steps, nil
	}

	return job.getJobStepsBuildPipeline(ctx, pipelinePod, pipelineJob)
}

func (job *Job) getJobStepsBuildPipeline(ctx context.Context, pipelinePod *corev1.Pod, pipelineJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	var steps []v1.RadixJobStep
	foundPrepareStepNames := make(map[string]bool)

	// Clone of radix config should be represented
	cloneConfigStep, applyConfigAndPreparePipelineStep := job.getCloneConfigApplyConfigAndPreparePipelineStep(ctx)
	if cloneConfigStep != nil {
		steps = append(steps, *cloneConfigStep)
		foundPrepareStepNames[cloneConfigStep.Name] = true
	}
	if applyConfigAndPreparePipelineStep != nil {
		steps = append(steps, *applyConfigAndPreparePipelineStep)
		foundPrepareStepNames[applyConfigAndPreparePipelineStep.Name] = true
	}

	// pipeline coordinator
	pipelineJobStep := getPipelineJobStep(pipelinePod)
	steps = append(steps, pipelineJobStep)

	jobStepsLabelSelector := fmt.Sprintf("%s=%s, %s!=%s", kube.RadixJobNameLabel, pipelineJob.GetName(), kube.RadixJobTypeLabel, kube.RadixJobTypeJob)
	jobStepList, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: jobStepsLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	for _, jobStep := range jobStepList.Items {
		jobType := jobStep.GetLabels()[kube.RadixJobTypeLabel]
		if strings.EqualFold(jobType, kube.RadixJobTypePreparePipelines) {
			// Prepare pipeline of radix config represented as an initial step
			continue
		}

		jobStepPod, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "job-name", jobStep.Name),
		})
		if err != nil {
			return nil, err
		}
		if len(jobStepPod.Items) == 0 {
			continue
		}

		pod := jobStepPod.Items[0]
		componentImages := make(pipeline.BuildComponentImages)
		if err := getObjectFromJobAnnotation(&jobStep, kube.RadixComponentImagesAnnotation, &componentImages); err != nil {
			return nil, err
		}
		buildComponents := make([]pipeline.BuildComponentImage, 0)
		if err := getObjectFromJobAnnotation(&jobStep, kube.RadixBuildComponentsAnnotation, &buildComponents); err != nil {
			return nil, err
		}
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			if strings.HasPrefix(containerStatus.Name, git.InternalContainerPrefix) {
				continue
			}
			if _, ok := foundPrepareStepNames[containerStatus.Name]; ok {
				continue
			}
			step := getJobStepWithNoComponents(pod.GetName(), &containerStatus)
			if containerStatus.Name == git.CloneContainerName {
				step.Components = getContainerNames(componentImages, buildComponents)
			}
			steps = append(steps, step)
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			components := getComponentImagesForContainer(containerStatus.Name, componentImages)
			components = append(components, getComponentsForContainer(containerStatus.Name, buildComponents)...)
			step := getJobStep(pod.GetName(), &containerStatus, components)
			if _, ok := foundPrepareStepNames[step.Name]; ok {
				continue
			}
			steps = append(steps, step)
		}
	}

	return steps, nil
}

func getContainerNames(buildComponentImagesMap pipeline.BuildComponentImages, buildComponentImagesList []pipeline.BuildComponentImage) []string {
	return append(maps.GetKeysFromMap(buildComponentImagesMap),
		slice.Map(buildComponentImagesList, func(componentImage pipeline.BuildComponentImage) string {
			return fmt.Sprintf("%s-%s", componentImage.ComponentName, componentImage.EnvName)
		})...)
}

func getObjectFromJobAnnotation(job *batchv1.Job, annotationName string, obj interface{}) error {
	annotation := job.GetObjectMeta().GetAnnotations()[annotationName]
	if !strings.EqualFold(annotation, "") {
		err := json.Unmarshal([]byte(annotation), obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func getComponentsForContainer(name string, componentImages []pipeline.BuildComponentImage) []string {
	components := make([]string, 0)
	for _, componentImage := range componentImages {
		if strings.EqualFold(componentImage.ContainerName, name) {
			components = append(components, componentImage.ComponentName)
		}
	}
	sort.Strings(components)
	return components
}

func getComponentImagesForContainer(name string, componentImages pipeline.BuildComponentImages) []string {
	components := make([]string, 0)
	for componentName, componentImage := range componentImages {
		if strings.EqualFold(componentImage.ContainerName, name) {
			components = append(components, componentName)
		}
	}
	sort.Strings(components)
	return components
}

func (job *Job) getPipelinePod(ctx context.Context) (*corev1.Pod, error) {
	pods, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "job-name", job.radixJob.Name),
	})

	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		// pipeline pod not found
		return nil, nil
	}

	return &pods.Items[0], nil
}

func getPipelineJobStep(pipelinePod *corev1.Pod) v1.RadixJobStep {
	var components []string
	return getJobStep(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0], components)
}

func (job *Job) getCloneConfigApplyConfigAndPreparePipelineStep(ctx context.Context) (*v1.RadixJobStep, *v1.RadixJobStep) {
	labelSelector := fmt.Sprintf("%s=%s, %s=%s", kube.RadixJobNameLabel, job.radixJob.GetName(), kube.RadixJobTypeLabel, kube.RadixJobTypePreparePipelines)

	preparePipelineStep, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil || len(preparePipelineStep.Items) != 1 {
		return nil, nil
	}

	jobPods, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "job-name", preparePipelineStep.Items[0].Name),
	})

	if err != nil || len(jobPods.Items) != 1 {
		return nil, nil
	}
	jobPod := jobPods.Items[0]

	cloneContainerStatus := getContainerStatusByName(git.CloneConfigContainerName, jobPod.Status.InitContainerStatuses)
	if cloneContainerStatus == nil {
		return nil, nil
	}
	cloneConfigContainerStep := getJobStepWithNoComponents(jobPods.Items[0].GetName(), cloneContainerStatus)
	applyConfigAndPreparePipelineContainerJobStep := getJobStepWithNoComponents(jobPod.GetName(), &jobPod.Status.ContainerStatuses[0])

	return &cloneConfigContainerStep, &applyConfigAndPreparePipelineContainerJobStep
}

func getContainerStatusByName(name string, containerStatuses []corev1.ContainerStatus) *corev1.ContainerStatus {
	for _, containerStatus := range containerStatuses {
		if containerStatus.Name == name {
			return &containerStatus
		}
	}

	return nil
}

func getJobStepWithNoComponents(podName string, containerStatus *corev1.ContainerStatus) v1.RadixJobStep {
	return getJobStepWithContainerName(podName, containerStatus.Name, containerStatus, nil)
}

func getJobStep(podName string, containerStatus *corev1.ContainerStatus, components []string) v1.RadixJobStep {
	return getJobStepWithContainerName(podName, containerStatus.Name, containerStatus, components)
}

func getJobStepWithContainerName(podName, containerName string, containerStatus *corev1.ContainerStatus, components []string) v1.RadixJobStep {
	var startedAt *metav1.Time
	var finishedAt *metav1.Time

	status := v1.JobSucceeded

	if containerStatus == nil {
		status = v1.JobWaiting

	} else if containerStatus.State.Terminated != nil {
		startedAt = &containerStatus.State.Terminated.StartedAt
		finishedAt = &containerStatus.State.Terminated.FinishedAt

		if containerStatus.State.Terminated.ExitCode > 0 {
			status = v1.JobFailed
		}

	} else if containerStatus.State.Running != nil {
		startedAt = &containerStatus.State.Running.StartedAt
		status = v1.JobRunning

	} else if containerStatus.State.Waiting != nil {
		status = v1.JobWaiting

	}

	return v1.RadixJobStep{
		Name:       containerName,
		Started:    startedAt,
		Ended:      finishedAt,
		Condition:  status,
		PodName:    podName,
		Components: components,
	}
}

func (job *Job) getJobEnvironments(ctx context.Context) ([]string, error) {
	deploymentsLinkedToJob, err := job.radixclient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "radix-job-name", job.radixJob.Name),
		})
	if err != nil {
		return nil, err
	}

	var environments []string
	for _, deployment := range deploymentsLinkedToJob.Items {
		environments = append(environments, deployment.Spec.Environment)
	}

	return environments, nil
}

func (job *Job) updateRadixJobStatusWithMetrics(ctx context.Context, savingRadixJob *v1.RadixJob, originalRadixJobCondition v1.RadixJobCondition, changeStatusFunc func(currStatus *v1.RadixJobStatus)) error {
	if err := job.updateRadixJobStatus(ctx, savingRadixJob, changeStatusFunc); err != nil {
		return err
	}
	if originalRadixJobCondition != job.radixJob.Status.Condition {
		metrics.RadixJobStatusChanged(job.radixJob)
	}
	return nil
}

func (job *Job) updateRadixJobStatus(ctx context.Context, rj *v1.RadixJob, changeStatusFunc func(currStatus *v1.RadixJobStatus)) error {
	rjInterface := job.radixclient.RadixV1().RadixJobs(rj.GetNamespace())
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentJob, err := rjInterface.Get(ctx, rj.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentJob.Status)
		_, err = rjInterface.UpdateStatus(ctx, currentJob, metav1.UpdateOptions{})

		if err == nil && rj.GetName() == job.radixJob.GetName() {
			currentJob, err = rjInterface.Get(ctx, rj.GetName(), metav1.GetOptions{})
			if err == nil {
				job.radixJob = currentJob
			}
		}
		return err
	})
	return err
}

func (job *Job) garbageCollectConfigMaps(ctx context.Context) {
	namespace := job.radixJob.GetNamespace()
	radixJobConfigMaps, err := job.kubeutil.ListConfigMapsWithSelector(ctx, namespace, getRadixJobNameExistsSelector().String())
	if err != nil {
		job.logger.Warn().Err(err).Msgf("Failed to get ConfigMaps while garbage collecting config-maps in %s", namespace)
		return
	}
	radixJobNameSet, err := job.getRadixJobNameSet(ctx)
	if err != nil {
		job.logger.Warn().Err(err).Msg("Failed to get RadixJob name set")
		return
	}
	for _, configMap := range radixJobConfigMaps {
		jobName := configMap.GetLabels()[kube.RadixJobNameLabel]
		if _, radixJobExists := radixJobNameSet[jobName]; !radixJobExists {
			job.logger.Debug().Msgf("Delete ConfigMap %s in %s", configMap.GetName(), configMap.GetNamespace())
			err := job.kubeutil.DeleteConfigMap(ctx, configMap.GetNamespace(), configMap.GetName())
			if err != nil {
				job.logger.Warn().Err(err).Msgf("failed to delete ConfigMap %s while garbage collecting config-maps in %s", configMap.GetName(), namespace)
			}
		}
	}
}

func (job *Job) getRadixJobNameSet(ctx context.Context) (map[string]bool, error) {
	radixJobs, err := job.getAllRadixJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RadixJobs: %w", err)
	}
	radixJobNameSet := make(map[string]bool)
	for _, radixJob := range radixJobs {
		radixJobNameSet[radixJob.GetName()] = true
	}
	return radixJobNameSet, nil
}

func getRadixJobNameExistsSelector() labels.Selector {
	requirement, _ := labels.NewRequirement(kube.RadixJobNameLabel, selection.Exists, []string{})
	return labels.NewSelector().Add(*requirement)
}
