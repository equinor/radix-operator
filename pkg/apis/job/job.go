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
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
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
}

const jobNameLabel = "job-name"

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
	}
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (job *Job) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).
		With().
		Str("resource_kind", v1.KindRadixJob).
		Str("resource_namespace", job.radixJob.GetNamespace()).
		Str("resource_name", job.radixJob.GetName()).
		Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

	job.restoreStatus(ctx)

	appName := job.radixJob.Spec.AppName
	ra, err := job.radixclient.RadixV1().RadixApplications(job.radixJob.GetNamespace()).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Ctx(ctx).Debug().Msgf("for BuildDeploy failed to find RadixApplication by name %s", appName)
	}

	if err := job.syncTargetEnvironments(ctx, ra); err != nil {
		return fmt.Errorf("failed to sync target environments: %w", err)
	}

	if IsRadixJobDone(job.radixJob) {
		log.Ctx(ctx).Debug().Msgf("Ignoring RadixJob %s/%s as it's no longer active.", job.radixJob.Namespace, job.radixJob.Name)
		return nil
	}

	stopReconciliation, err := job.syncStatuses(ctx, ra)
	if err != nil {
		return err
	}
	if stopReconciliation {
		log.Ctx(ctx).Info().Msgf("stop reconciliation, status updated triggering new sync")
		return nil
	}

	_, err = job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(ctx, job.radixJob.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if err = job.createPipelineJob(ctx); err != nil {
			return err
		}
		log.Ctx(ctx).Debug().Msg("RadixJob created")
		if err = job.setStatusOfJob(ctx); err != nil {
			return err
		}
	}
	return nil
}

// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
func (job *Job) restoreStatus(ctx context.Context) {
	if restoredStatus, ok := job.radixJob.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if job.radixJob.Status.Condition == "" {
			var status v1.RadixJobStatus
			err := json.Unmarshal([]byte(restoredStatus), &status)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Unable to get status from annotation")
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
				log.Ctx(ctx).Error().Err(err).Msg("Unable to restore status")
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

	appRadixJobs, err := job.getRadixJobs(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get all RadixJobs: %v", err)
		return false, err
	}
	if job.isOtherJobRunningOnBranchOrEnvironment(ra, appRadixJobs) {
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

func (job *Job) isOtherJobRunningOnBranchOrEnvironment(ra *v1.RadixApplication, appRadixJobs []v1.RadixJob) bool {
	isJobActive := func(rj *v1.RadixJob) bool {
		return rj.Status.Condition == v1.JobWaiting || rj.Status.Condition == v1.JobRunning
	}

	jobTargetEnvironments := job.getTargetEnvironments(ra)
	for _, rj := range appRadixJobs {
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

func (job *Job) getTargetEnvironments(ra *v1.RadixApplication) map[string]struct{} {
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
	log.Ctx(ctx).Debug().Msg("Set RadixJob status")
	pipelineJobs, err := job.getPipelineJobs(ctx)
	if err != nil {
		return err
	}
	pipelineJob, pipelineJobExists := slice.FindFirst(pipelineJobs, func(j batchv1.Job) bool { return j.GetName() == job.radixJob.Name })
	if !pipelineJobExists {
		log.Ctx(ctx).Debug().Msg("Pipeline job does not yet exist, nothing to sync")
		return nil
	}
	log.Ctx(ctx).Debug().Msg("Get RadixJob steps")
	steps, err := job.getJobSteps(ctx, pipelineJobs)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Debug().Msgf("Got %d RadixJob steps", len(steps))
	environments, err := job.getJobEnvironments(ctx)
	if err != nil {
		return err
	}
	jobStatusCondition, err := job.getJobConditionFromJobStatus(ctx, pipelineJob.Status)
	if err != nil {
		return err
	}

	err = job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
		log.Ctx(ctx).Debug().Msgf("Update RadixJob status with %d steps and the condition %s", len(steps), jobStatusCondition)
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

	err = job.deleteJobPodIfExistsAndNotCompleted(ctx, job.radixJob.Name)
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
	jobList, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s!=%s", kube.RadixJobNameLabel, job.radixJob.Name, kube.RadixJobTypeLabel, kube.RadixJobTypeJob),
	})

	if err != nil {
		return err
	}
	if len(jobList.Items) == 0 {
		return nil
	}
	for _, kubernetesJob := range jobList.Items {
		if kubernetesJob.Status.CompletionTime != nil {
			continue
		}
		if err = job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Delete(ctx, kubernetesJob.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err = job.deleteJobPodIfExistsAndNotCompleted(ctx, kubernetesJob.Name); err != nil {
			return err
		}
	}
	return nil
}

func (job *Job) setNextJobToRunning(ctx context.Context) error {
	radixJobs, err := job.getAllRadixJobs(ctx)
	if err != nil {
		return err
	}

	rjs := sortRadixJobsByCreatedAsc(radixJobs)
	for _, otherRj := range rjs {
		if otherRj.Name != job.radixJob.Name && otherRj.Status.Condition == v1.JobQueued { // previous status for this otherRj was Queued
			return job.updateRadixJobStatusWithMetrics(ctx, &otherRj, v1.JobQueued, func(currStatus *v1.RadixJobStatus) {
				currStatus.Condition = v1.JobRunning
			})
		}
	}
	return nil
}

func (job *Job) getAllRadixJobs(ctx context.Context) ([]v1.RadixJob, error) {
	jobList, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.GetNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return jobList.Items, err
}

func (job *Job) queueJob(ctx context.Context) error {
	return job.updateRadixJobStatusWithMetrics(ctx, job.radixJob, job.originalRadixJobCondition, func(currStatus *v1.RadixJobStatus) {
		currStatus.Created = &job.radixJob.CreationTimestamp
		currStatus.Condition = v1.JobQueued
	})
}

func (job *Job) getPipelineJobsPods(ctx context.Context) ([]corev1.Pod, error) {
	podList, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: job.getRadixJobNameLabelSelector(),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func (job *Job) getJobSteps(ctx context.Context, pipelineJobs []batchv1.Job) ([]v1.RadixJobStep, error) {
	pipelineJobsPods, err := job.getPipelineJobsPods(ctx)
	if err != nil {
		return nil, err
	}
	pipelineOrchestratorPod := job.getPipelineOrchestratorPod(pipelineJobsPods)
	if pipelineOrchestratorPod == nil || len(pipelineOrchestratorPod.Spec.Containers) == 0 {
		log.Ctx(ctx).Debug().Msg("Pipeline orchestrator job pod or its container does not yet exist, skip steps build")
		return nil, nil
	}

	var steps []v1.RadixJobStep

	steps = append(steps, getOrchestratorStep(pipelineOrchestratorPod))

	preparePipelineSteps, prepareStepsNames := job.getPreparePipelineSteps(pipelineJobs, pipelineJobsPods)
	steps = append(steps, preparePipelineSteps...)

	pipelineSteps, err := getPipelineSteps(ctx, pipelineJobs, pipelineJobsPods, prepareStepsNames)
	if err != nil {
		return nil, err
	}
	steps = append(steps, pipelineSteps...)

	return steps, nil
}

func getOrchestratorStep(pod *corev1.Pod) v1.RadixJobStep {
	containerName := defaults.RadixPipelineJobPipelineContainerName
	containerStatus := getContainerStatusForContainer(pod, containerName)
	return getJobStep(pod.GetName(), containerName, containerStatus)
}

func getPipelineSteps(ctx context.Context, jobs []batchv1.Job, jobPods []corev1.Pod, prepareStepsNames map[string]struct{}) ([]v1.RadixJobStep, error) {
	var steps []v1.RadixJobStep
	stepJobs := slice.FindAll(jobs, func(job batchv1.Job) bool {
		return job.GetLabels()[kube.RadixJobTypeLabel] != kube.RadixJobTypeJob
	})
	for _, jobStep := range stepJobs {
		jobType := jobStep.GetLabels()[kube.RadixJobTypeLabel]
		if strings.EqualFold(jobType, kube.RadixJobTypePreparePipelines) {
			// Prepare pipeline of radix config represented as an initial step
			continue
		}

		pod, podExists := slice.FindFirst(jobPods, func(jobPod corev1.Pod) bool {
			return jobPod.GetLabels()[jobNameLabel] == jobStep.Name
		})
		if !podExists {
			log.Ctx(ctx).Debug().Msgf("Skip the RadixJob step %s - job pod does not exist", jobStep.Name)
			continue
		}

		buildComponentImagesMap, buildComponentImages, err := getBuildComponentImagesFromJobAnnotations(&jobStep)
		if err != nil {
			return nil, err
		}

		log.Ctx(ctx).Debug().Msgf("Add %d steps for the job %s", len(pod.Spec.Containers), jobStep.Name)
		steps = append(steps, getJobStepsFromInitContainers(ctx, jobStep, pod, prepareStepsNames, buildComponentImagesMap, buildComponentImages)...)
		steps = append(steps, getJobStepsFromContainers(ctx, jobStep, pod, buildComponentImagesMap, buildComponentImages, prepareStepsNames)...)
	}
	return steps, nil
}

func (job *Job) getPreparePipelineSteps(jobs []batchv1.Job, jobsPods []corev1.Pod) ([]v1.RadixJobStep, map[string]struct{}) {
	var steps []v1.RadixJobStep
	prepareStepsNames := make(map[string]struct{})

	cloneConfigStep, applyConfigAndPreparePipelineStep := job.getCloneConfigApplyConfigAndPreparePipelineStep(jobs, jobsPods)
	if cloneConfigStep != nil {
		steps = append(steps, *cloneConfigStep)
		prepareStepsNames[cloneConfigStep.Name] = struct{}{}
	}
	if applyConfigAndPreparePipelineStep != nil {
		steps = append(steps, *applyConfigAndPreparePipelineStep)
		prepareStepsNames[applyConfigAndPreparePipelineStep.Name] = struct{}{}
	}
	return steps, prepareStepsNames
}

func getBuildComponentImagesFromJobAnnotations(job *batchv1.Job) (pipeline.BuildComponentImages, []pipeline.BuildComponentImage, error) {
	buildComponentImagesMap := make(pipeline.BuildComponentImages)
	if err := getObjectFromJobAnnotation(job, kube.RadixComponentImagesAnnotation, &buildComponentImagesMap); err != nil {
		return nil, nil, err
	}
	buildComponentImages := make([]pipeline.BuildComponentImage, 0)
	if err := getObjectFromJobAnnotation(job, kube.RadixBuildComponentsAnnotation, &buildComponentImages); err != nil {
		return nil, nil, err
	}
	return buildComponentImagesMap, buildComponentImages, nil
}

func getJobStepsFromContainers(ctx context.Context, jobStep batchv1.Job, pod corev1.Pod, buildComponentImagesMap pipeline.BuildComponentImages, buildComponentImages []pipeline.BuildComponentImage, prepareStepsNames map[string]struct{}) []v1.RadixJobStep {
	containerStatuses := slice.Reduce(pod.Status.ContainerStatuses, make(map[string]*corev1.ContainerStatus), func(acc map[string]*corev1.ContainerStatus, containerStatus corev1.ContainerStatus) map[string]*corev1.ContainerStatus {
		acc[containerStatus.Name] = &containerStatus
		return acc
	})
	var steps []v1.RadixJobStep
	for _, container := range pod.Spec.Containers {
		if _, ok := prepareStepsNames[container.Name]; ok {
			continue
		}
		componentNames := getComponentNamesForBuildComponentImagesMap(container.Name, buildComponentImagesMap)
		componentNames = append(componentNames, getComponentNamesForBuildComponentImages(container.Name, buildComponentImages)...)
		steps = append(steps, getJobStep(pod.GetName(), container.Name, containerStatuses[container.Name], componentNames...))
		log.Ctx(ctx).Debug().Msgf("Added step %s for the job %s", container.Name, jobStep.Name)
	}
	return steps
}

func getJobStepsFromInitContainers(ctx context.Context, jobStep batchv1.Job, pod corev1.Pod, prepareStepsNames map[string]struct{}, buildComponentImagesMap pipeline.BuildComponentImages, buildComponentImages []pipeline.BuildComponentImage) []v1.RadixJobStep {
	containerStatuses := slice.Reduce(pod.Status.InitContainerStatuses, make(map[string]*corev1.ContainerStatus), func(acc map[string]*corev1.ContainerStatus, containerStatus corev1.ContainerStatus) map[string]*corev1.ContainerStatus {
		acc[containerStatus.Name] = &containerStatus
		return acc
	})
	var steps []v1.RadixJobStep
	for _, container := range pod.Spec.InitContainers {
		if _, ok := prepareStepsNames[container.Name]; ok || strings.HasPrefix(container.Name, git.InternalContainerPrefix) {
			continue
		}
		var componentNames []string
		if container.Name == git.CloneContainerName {
			componentNames = getContainerNames(buildComponentImagesMap, buildComponentImages)
		}
		steps = append(steps, getJobStep(pod.GetName(), container.Name, containerStatuses[container.Name], componentNames...))
		log.Ctx(ctx).Debug().Msgf("Added step %s for the job %s", container.Name, jobStep.Name)
	}
	return steps
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

func getComponentNamesForBuildComponentImages(containerName string, componentImages []pipeline.BuildComponentImage) []string {
	componentNames := make([]string, 0)
	for _, componentImage := range componentImages {
		if strings.EqualFold(componentImage.ContainerName, containerName) {
			componentNames = append(componentNames, componentImage.ComponentName)
		}
	}
	sort.Strings(componentNames)
	return componentNames
}

func getComponentNamesForBuildComponentImagesMap(containerName string, componentImages pipeline.BuildComponentImages) []string {
	componentNames := make([]string, 0)
	for componentName, componentImage := range componentImages {
		if strings.EqualFold(componentImage.ContainerName, containerName) {
			componentNames = append(componentNames, componentName)
		}
	}
	sort.Strings(componentNames)
	return componentNames
}

func (job *Job) getPipelineOrchestratorPod(pipelineJobs []corev1.Pod) *corev1.Pod {
	if jobPod, ok := slice.FindFirst(pipelineJobs, func(pod corev1.Pod) bool { return pod.GetLabels()[jobNameLabel] == job.radixJob.Name }); ok {
		return &jobPod
	}
	return nil
}

func getContainerStatusForContainer(pipelinePod *corev1.Pod, containerName string) *corev1.ContainerStatus {
	if containerStatus, ok := slice.FindFirst(pipelinePod.Status.ContainerStatuses, func(containerStatus corev1.ContainerStatus) bool { return containerStatus.Name == containerName }); ok {
		return &containerStatus
	}
	return nil
}

func (job *Job) getCloneConfigApplyConfigAndPreparePipelineStep(pipelineJobs []batchv1.Job, pipelineJobsPods []corev1.Pod) (*v1.RadixJobStep, *v1.RadixJobStep) {
	preparePipelineStepJob, jobExists := slice.FindFirst(pipelineJobs, func(job batchv1.Job) bool {
		return job.GetLabels()[kube.RadixJobTypeLabel] == kube.RadixJobTypePreparePipelines
	})
	if !jobExists {
		return nil, nil
	}

	jobPod, jobPodExists := slice.FindFirst(pipelineJobsPods, func(pod corev1.Pod) bool { return pod.GetLabels()[jobNameLabel] == preparePipelineStepJob.GetName() })
	if !jobPodExists {
		return nil, nil
	}

	cloneContainerStatus := getContainerStatusByName(git.CloneConfigContainerName, jobPod.Status.InitContainerStatuses)
	cloneConfigContainerStep := getJobStep(jobPod.GetName(), git.CloneConfigContainerName, cloneContainerStatus)

	preparePipelineContainerStatus := getContainerStatusByName(defaults.RadixPipelineJobPreparePipelinesContainerName, jobPod.Status.ContainerStatuses)
	applyConfigAndPreparePipelineContainerJobStep := getJobStep(jobPod.GetName(), defaults.RadixPipelineJobPreparePipelinesContainerName, preparePipelineContainerStatus)

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

func getJobStep(podName, containerName string, containerStatus *corev1.ContainerStatus, components ...string) v1.RadixJobStep {
	step := v1.RadixJobStep{
		Name:       containerName,
		PodName:    podName,
		Components: components,
		Condition:  v1.JobSucceeded,
	}
	switch {
	case containerStatus == nil:
		step.Condition = v1.JobWaiting
	case containerStatus.State.Terminated != nil:
		step.Started = &containerStatus.State.Terminated.StartedAt
		step.Ended = &containerStatus.State.Terminated.FinishedAt
		if containerStatus.State.Terminated.ExitCode > 0 {
			step.Condition = v1.JobFailed
		}
	case containerStatus.State.Running != nil:
		step.Started = &containerStatus.State.Running.StartedAt
		step.Condition = v1.JobRunning
	case containerStatus.State.Waiting != nil:
		step.Condition = v1.JobWaiting
	}
	return step
}

func (job *Job) getJobEnvironments(ctx context.Context) ([]string, error) {
	deploymentsLinkedToJob, err := job.radixclient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(
		ctx,
		metav1.ListOptions{LabelSelector: job.getRadixJobNameLabelSelector()})
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
		log.Ctx(ctx).Debug().Msg("UpdateRadixJobStatus")
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

func (job *Job) getPipelineJobs(ctx context.Context) ([]batchv1.Job, error) {
	jobList, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: job.getRadixJobNameLabelSelector(),
	})
	if err != nil {
		return nil, err
	}
	return jobList.Items, nil
}

func (job *Job) getRadixJobNameLabelSelector() string {
	return labels.Set{kube.RadixJobNameLabel: job.radixJob.Name}.String()
}

func (job *Job) getRadixJobs(ctx context.Context) ([]v1.RadixJob, error) {
	radixJobList, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return radixJobList.Items, err
}

func (job *Job) deleteJobPodIfExistsAndNotCompleted(ctx context.Context, jobName string) error {
	pods, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.Set{jobNameLabel: jobName}.String()})
	if err != nil {
		return err
	}
	if pods == nil || len(pods.Items) == 0 || pods.Items[0].Status.Phase == corev1.PodSucceeded || pods.Items[0].Status.Phase == corev1.PodFailed {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).Delete(ctx, pods.Items[0].Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil

}
