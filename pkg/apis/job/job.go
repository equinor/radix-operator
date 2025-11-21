package job

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	commonmaps "github.com/equinor/radix-common/utils/maps"
	commonslice "github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// Job Instance variables
type Job struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	radixJob     *v1.RadixJob
	registration *v1.RadixRegistration
	config       *apiconfig.Config
}

const jobNameLabel = "job-name"

// NewJob Constructor
func NewJob(kubeClient kubernetes.Interface, kubeUtil *kube.Kube, radixClient radixclient.Interface, registration *v1.RadixRegistration, radixJob *v1.RadixJob, config *apiconfig.Config) *Job {
	return &Job{
		kubeclient:   kubeClient,
		radixclient:  radixClient,
		kubeutil:     kubeUtil,
		registration: registration,
		radixJob:     radixJob,
		config:       config,
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

	appName := job.radixJob.Spec.AppName
	ra, err := job.radixclient.RadixV1().RadixApplications(job.radixJob.GetNamespace()).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Ctx(ctx).Debug().Msgf("for BuildDeploy failed to find RadixApplication by name %s", appName)
	}

	if job.radixJob.Status.Condition.IsDoneCondition() {
		log.Ctx(ctx).Debug().Msgf("Ignoring RadixJob %s/%s as it's no longer active.", job.radixJob.Namespace, job.radixJob.Name)
		return nil
	}

	if err := job.syncTargetEnvironments(ctx, ra); err != nil {
		return fmt.Errorf("failed to sync target environments: %w", err)
	}

	if stopped, err := job.handleStop(ctx, ra); err != nil {
		return err
	} else if stopped {
		return nil
	}

	if queued, err := job.handleJobQueueing(ctx, ra); err != nil {
		return err
	} else if queued {
		return nil
	}

	if err := job.syncStatus(ctx, job.reconcile(ctx)); err != nil {
		return err
	}

	if job.radixJob.Status.Condition.IsDoneCondition() {
		if err = job.setNextJobToRunning(ctx, ra); err != nil {
			return err
		}
	}

	return nil
}

// handleStop stops the job if the Stop flag is set and activates the next queued job.
// Returns true if the job was stopped, indicating that reconciliation should stop.
func (job *Job) handleStop(ctx context.Context, ra *v1.RadixApplication) (bool, error) {
	if !job.radixJob.Spec.Stop {
		return false, nil
	}

	if err := job.stopJob(ctx); err != nil {
		return false, fmt.Errorf("failed to stop job: %w", err)
	}

	if err := job.setNextJobToRunning(ctx, ra); err != nil {
		return false, fmt.Errorf("failed to set next job to running: %w", err)
	}

	return true, nil
}

func (job *Job) reconcile(ctx context.Context) error {
	_, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(ctx, job.radixJob.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Ctx(ctx).Info().Msg("Create pipeline job")
			return job.createPipelineJob(ctx)
		}

		return fmt.Errorf("failed to create pipeline job: %s", err)
	}

	return nil
}

// handleJobQueueing checks if another job is running on the same branch or environment and queues this job if necessary.
// Returns true if the job was queued, indicating that reconciliation should stop.
func (job *Job) handleJobQueueing(ctx context.Context, ra *v1.RadixApplication) (bool, error) {
	if !slices.Contains([]v1.RadixJobCondition{"", v1.JobQueued}, job.radixJob.Status.Condition) {
		return false, nil
	}

	existingRadixJobs, err := job.getRadixJobs(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list jobs: %w", err)
	}

	if isOtherJobRunningOnBranchOrEnvironment(ra, job.radixJob, existingRadixJobs) {
		if err := job.queueJob(ctx); err != nil {
			return false, fmt.Errorf("failed to queue job: %w", err)
		}

		return true, nil
	}

	return false, nil
}

func isOtherJobRunningOnBranchOrEnvironment(ra *v1.RadixApplication, job *v1.RadixJob, existingRadixJobs []v1.RadixJob) bool {
	isJobActive := func(rj *v1.RadixJob) bool {
		return rj.Status.Condition == v1.JobWaiting || rj.Status.Condition == v1.JobRunning
	}

	jobTargetEnvironments := getTargetEnvironments(job, ra)
	for _, existingRadixJob := range existingRadixJobs {
		if existingRadixJob.GetName() == job.GetName() || !isJobActive(&existingRadixJob) {
			continue
		}
		switch existingRadixJob.Spec.PipeLineType {
		case v1.BuildDeploy, v1.Build:
			if len(jobTargetEnvironments) > 0 {
				existingJobTargetEnvironments := applicationconfig.GetAllTargetEnvironments(existingRadixJob.Spec.Build.GetGitRefOrDefault(), string(existingRadixJob.Spec.Build.GitRefType), ra)
				for _, existingJobTargetEnvironment := range existingJobTargetEnvironments {
					if _, ok := jobTargetEnvironments[existingJobTargetEnvironment]; ok {
						return true
					}
				}
				continue
			}
			if job.Spec.Build.GetGitRefOrDefault() == existingRadixJob.Spec.Build.GetGitRefOrDefault() &&
				job.Spec.Build.GetGitRefTypeOrDefault() == existingRadixJob.Spec.Build.GetGitRefTypeOrDefault() {
				if len(job.Spec.Build.ToEnvironment) == 0 {
					return true
				}
				if _, ok := jobTargetEnvironments[existingRadixJob.Spec.Build.ToEnvironment]; ok {
					return true
				}
			}
		case v1.Deploy:
			if _, ok := jobTargetEnvironments[existingRadixJob.Spec.Deploy.ToEnvironment]; ok {
				return true
			}
		case v1.Promote:
			if _, ok := jobTargetEnvironments[existingRadixJob.Spec.Promote.ToEnvironment]; ok {
				return true
			}
		}
	}
	return false
}

func getTargetEnvironments(rj *v1.RadixJob, ra *v1.RadixApplication) map[string]any {
	switch rj.Spec.PipeLineType {
	case v1.Build, v1.BuildDeploy:
		environments, _, _ := applicationconfig.GetTargetEnvironments(rj.Spec.Build.GetGitRefOrDefault(), string(rj.Spec.Build.GitRefType), ra, rj.Spec.TriggeredFromWebhook)
		return commonslice.Reduce(environments,
			make(map[string]any), func(acc map[string]any, envName string) map[string]any {
				if len(rj.Spec.Build.ToEnvironment) == 0 || envName == rj.Spec.Build.ToEnvironment {
					acc[envName] = nil
				}
				return acc
			})
	case v1.Deploy:
		return map[string]any{rj.Spec.Deploy.ToEnvironment: nil}
	case v1.Promote:
		return map[string]any{rj.Spec.Promote.ToEnvironment: nil}
	default:
		return nil
	}
}

// sync the environments in the RadixJob with environments in the RA
func (job *Job) syncTargetEnvironments(ctx context.Context, ra *v1.RadixApplication) error {
	// TargetEnv has already been set
	if len(job.radixJob.Status.TargetEnvs) > 0 {
		return nil
	}

	// Update RJ with accurate env data
	return job.updateStatus(ctx, func(currStatus *v1.RadixJobStatus) {
		currStatus.TargetEnvs = slices.Collect(maps.Keys(getTargetEnvironments(job.radixJob, ra)))
	})
}

func (job *Job) syncStatus(ctx context.Context, reconcileErr error) error {
	log.Ctx(ctx).Debug().Msg("Set RadixJob status")
	pipelineJobs, err := job.getPipelineJobs(ctx)
	if err != nil {
		return err
	}
	pipelineJob, pipelineJobExists := commonslice.FindFirst(pipelineJobs, func(j batchv1.Job) bool { return j.GetName() == job.radixJob.Name })
	if !pipelineJobExists {
		log.Ctx(ctx).Debug().Msg("Pipeline job does not yet exist, nothing to sync")
		return reconcileErr
	}
	log.Ctx(ctx).Debug().Msg("Get RadixJob steps")
	steps, err := job.getJobSteps(ctx, pipelineJobs)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Debug().Msgf("Got %d RadixJob steps", len(steps))
	jobStatusCondition, err := job.getJobConditionFromJobStatus(ctx, pipelineJob.Status)
	if err != nil {
		return err
	}

	environments, err := job.getJobEnvironments(ctx)
	if err != nil {
		return err
	}
	if err = job.updateStatus(ctx, func(currStatus *v1.RadixJobStatus) {
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

		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = job.radixJob.Generation
		if reconcileErr != nil {
			currStatus.ReconcileStatus = v1.RadixJobReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = v1.RadixJobReconcileSucceeded
			currStatus.Message = ""
		}
	}); err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}

	return reconcileErr
}

func (job *Job) stopJob(ctx context.Context) error {
	log.Ctx(ctx).Info().Msgf("Stop job")
	isRunning := job.radixJob.Status.Condition == v1.JobRunning
	stoppedSteps, err := job.getStoppedSteps(ctx, isRunning)
	if err != nil {
		return err
	}

	return job.updateStatus(ctx, func(currStatus *v1.RadixJobStatus) {
		if isRunning && stoppedSteps != nil {
			currStatus.Steps = *stoppedSteps
		}
		currStatus.Created = &job.radixJob.CreationTimestamp
		currStatus.Condition = v1.JobStopped
		currStatus.Ended = &metav1.Time{Time: time.Now()}

		currStatus.Reconciled = metav1.Now()
		currStatus.ReconcileStatus = v1.RadixJobReconcileSucceeded
		currStatus.ObservedGeneration = job.radixJob.Generation
	})

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

func (job *Job) setNextJobToRunning(ctx context.Context, ra *v1.RadixApplication) error {
	radixJobs, err := job.getAllRadixJobs(ctx)
	if err != nil {
		return err
	}

	rjs := sortRadixJobsByCreatedAsc(radixJobs)
	for _, thisJob := range rjs {
		if thisJob.Status.Condition != v1.JobQueued {
			continue
		}
		if !isOtherJobRunningOnBranchOrEnvironment(ra, &thisJob, rjs) {
			log.Ctx(ctx).Info().Msgf("Change condition for next job %s from %s to %s", thisJob.Name, thisJob.Status.Condition, v1.JobRunning)
			_, err := updateRadixJobStatus(ctx, job.radixclient, &thisJob, func(currStatus *v1.RadixJobStatus) {
				currStatus.Condition = v1.JobRunning
			})
			return err
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
	log.Ctx(ctx).Info().Msg("Queue job")
	return job.updateStatus(ctx, func(currStatus *v1.RadixJobStatus) {
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
	prepareStepsNames := make(map[string]struct{})

	cloneConfigStep := job.getPreparePipelineSteps(pipelineOrchestratorPod)
	steps = append(steps, cloneConfigStep)
	prepareStepsNames[cloneConfigStep.Name] = struct{}{}

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
	jobStep := getJobStep(pod.GetName(), containerName, containerStatus)
	return jobStep
}

func getPipelineSteps(ctx context.Context, jobs []batchv1.Job, jobPods []corev1.Pod, prepareStepsNames map[string]struct{}) ([]v1.RadixJobStep, error) {
	var steps []v1.RadixJobStep
	stepJobs := commonslice.FindAll(jobs, func(job batchv1.Job) bool {
		return job.GetLabels()[kube.RadixJobTypeLabel] != kube.RadixJobTypeJob
	})
	for _, jobStep := range stepJobs {
		pod, podExists := commonslice.FindFirst(jobPods, func(jobPod corev1.Pod) bool {
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
		steps = append(steps, getJobStepsFromInitContainers(pod, prepareStepsNames, buildComponentImagesMap, buildComponentImages)...)
		steps = append(steps, getJobStepsFromContainers(ctx, jobStep, pod, buildComponentImagesMap, buildComponentImages, prepareStepsNames)...)
	}
	return steps, nil
}

func (job *Job) getPreparePipelineSteps(jobPod *corev1.Pod) v1.RadixJobStep {
	cloneContainerStatus := getContainerStatusByName(git.CloneConfigContainerName, jobPod.Status.InitContainerStatuses)
	cloneConfigStep := getJobStep(jobPod.GetName(), git.CloneConfigContainerName, cloneContainerStatus)
	return cloneConfigStep
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
	containerStatuses := commonslice.Reduce(pod.Status.ContainerStatuses, make(map[string]*corev1.ContainerStatus), func(acc map[string]*corev1.ContainerStatus, containerStatus corev1.ContainerStatus) map[string]*corev1.ContainerStatus {
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

func getJobStepsFromInitContainers(pod corev1.Pod, prepareStepsNames map[string]struct{}, buildComponentImagesMap pipeline.BuildComponentImages, buildComponentImages []pipeline.BuildComponentImage) []v1.RadixJobStep {
	containerStatuses := commonslice.Reduce(pod.Status.InitContainerStatuses, make(map[string]*corev1.ContainerStatus), func(acc map[string]*corev1.ContainerStatus, containerStatus corev1.ContainerStatus) map[string]*corev1.ContainerStatus {
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
	}
	return steps
}

func getContainerNames(buildComponentImagesMap pipeline.BuildComponentImages, buildComponentImagesList []pipeline.BuildComponentImage) []string {
	return append(commonmaps.GetKeysFromMap(buildComponentImagesMap),
		commonslice.Map(buildComponentImagesList, func(componentImage pipeline.BuildComponentImage) string {
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
	if jobPod, ok := commonslice.FindFirst(pipelineJobs, func(pod corev1.Pod) bool { return pod.GetLabels()[jobNameLabel] == job.radixJob.Name }); ok {
		return &jobPod
	}
	return nil
}

func getContainerStatusForContainer(pipelinePod *corev1.Pod, containerName string) *corev1.ContainerStatus {
	if containerStatus, ok := commonslice.FindFirst(pipelinePod.Status.ContainerStatuses, func(containerStatus corev1.ContainerStatus) bool { return containerStatus.Name == containerName }); ok {
		return &containerStatus
	}
	return nil
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
	radixDeployments, err := job.kubeutil.GetRadixDeploymentsForApp(ctx, job.radixJob.Spec.AppName, job.getRadixJobNameLabelSelector())
	if err != nil {
		return nil, err
	}
	environmentsMap := commonslice.Reduce(radixDeployments, make(map[string]struct{}), func(acc map[string]struct{}, rd v1.RadixDeployment) map[string]struct{} {
		if rd.GetLabels()[kube.RadixJobNameLabel] == job.radixJob.Name {
			acc[rd.Spec.Environment] = struct{}{}
		}
		return acc
	})
	return commonmaps.GetKeysFromMap(environmentsMap), nil
}

func (job *Job) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *v1.RadixJobStatus)) error {
	updatedRJ, err := updateRadixJobStatus(ctx, job.radixclient, job.radixJob, changeStatusFunc)
	if err != nil {
		return err
	}
	job.radixJob = updatedRJ
	return nil
}

func updateRadixJobStatus(ctx context.Context, client radixclient.Interface, rj *v1.RadixJob, changeStatusFunc func(currStatus *v1.RadixJobStatus)) (*v1.RadixJob, error) {
	updateObj := rj.DeepCopy()
	changeStatusFunc(&updateObj.Status)
	updateObj, err := client.RadixV1().RadixJobs(rj.GetNamespace()).UpdateStatus(ctx, updateObj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if rj.Status.Condition != updateObj.Status.Condition {
		metrics.RadixJobStatusChanged(updateObj)
	}
	return updateObj, nil
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
