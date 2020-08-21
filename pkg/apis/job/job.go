package job

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/metrics"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/prometheus/common/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Job Instance variables
type Job struct {
	kubeclient             kubernetes.Interface
	radixclient            radixclient.Interface
	kubeutil               *kube.Kube
	radixJob               *v1.RadixJob
	originalRadixJobStatus v1.RadixJobCondition
}

// NewJob Constructor
func NewJob(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	radixJob *v1.RadixJob) Job {
	originalRadixJobStatus := radixJob.Status.Condition

	return Job{
		kubeclient,
		radixclient,
		kubeutil, radixJob, originalRadixJobStatus}
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (job *Job) OnSync() error {
	job.restoreStatus()
	job.AddTargetEnvironments()
	job.SyncTargetEnvironments()

	if IsRadixJobDone(job.radixJob) {
		log.Debugf("Ignoring RadixJob %s/%s as it's no longer active.", job.radixJob.Namespace, job.radixJob.Name)
		return nil
	}

	stopReconciliation, err := job.syncStatuses()
	if err != nil {
		return err
	}
	if stopReconciliation {
		log.Infof("stop reconciliation, status updated triggering new sync")
		return nil
	}

	_, err = job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(job.radixJob.Name, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		err = job.createJob()
	}

	job.maintainHistoryLimit()

	return err
}

// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
func (job *Job) restoreStatus() {
	if restoredStatus, ok := job.radixJob.Annotations[kube.RestoredStatusAnnotation]; ok {
		if job.radixJob.Status.Condition == "" {
			var status v1.RadixJobStatus
			err := json.Unmarshal([]byte(restoredStatus), &status)
			if err != nil {
				log.Error("Unable to get status from annotation", err)
				return
			}

			job.radixJob.Status = status
			err = saveStatus(job.radixclient, job.radixJob, job.originalRadixJobStatus)
			if err != nil {
				log.Error("Unable to restore status", err)
				return
			}
		}
	}
}

func (job *Job) syncStatuses() (stopReconciliation bool, err error) {
	stopReconciliation = false

	if job.radixJob.Spec.Stop {
		err = job.stopJob()
		if err != nil {
			return false, err
		}

		return true, nil
	}

	allJobs, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("Failed to get all RadixJobs. Error was %v", err)
		return false, err
	}

	if job.isOtherJobRunningOnBranch(allJobs.Items) {
		err = job.queueJob()
		if err != nil {
			return false, err
		}

		return true, nil
	}

	err = job.setStatusOfJob()
	if err != nil {
		return false, err
	}

	return
}

func (job *Job) isOtherJobRunningOnBranch(allJobs []v1.RadixJob) bool {
	for _, rj := range allJobs {
		if rj.GetName() != job.radixJob.GetName() &&
			job.radixJob.Spec.Build.Branch != "" &&
			job.radixJob.Spec.Build.Branch == rj.Spec.Build.Branch &&
			rj.Status.Condition == v1.JobRunning {
			return true
		}
	}

	return false
}

// AddTargetEnvironments read environments from RadixApplication and updates the status of RadixJob
func (job *Job) AddTargetEnvironments() {
	rj := job.radixJob
	radixApplication, err := job.radixclient.RadixV1().RadixApplications(rj.Namespace).Get(rj.Spec.AppName, metav1.GetOptions{})
	var targetEnvs []string

	if err != nil {
		targetEnvs = append(targetEnvs, "N/A")
	} else {
		for _, env := range radixApplication.Spec.Environments {
			targetEnvs = append(targetEnvs, env.Name)
		}
	}

	rj.Status.TargetEnvs = targetEnvs
	job.radixclient.RadixV1().RadixJobs(rj.Namespace).UpdateStatus(rj)

}

// SyncTargetEnvironments sync the environments in the RadixJob with environments in the RA
func (job *Job) SyncTargetEnvironments() {
	rj := job.radixJob
	radixApplication, err := job.radixclient.RadixV1().RadixApplications(rj.Namespace).Get(rj.Spec.AppName, metav1.GetOptions{})

	if err == nil {
		// Radix application exists
		var raEnvsSlice []string
		for _, env := range radixApplication.Spec.Environments {
			raEnvsSlice = append(raEnvsSlice, env.Name)
		}

		if !reflect.DeepEqual(rj.Status.TargetEnvs, raEnvsSlice) {
			job.radixJob.Status.TargetEnvs = raEnvsSlice
			job.radixclient.RadixV1().RadixJobs(rj.Namespace).UpdateStatus(rj)
		}
	}

}

// IsRadixJobDone Checks if job is done
func IsRadixJobDone(rj *v1.RadixJob) bool {
	return rj == nil || rj.Status.Condition == v1.JobFailed || rj.Status.Condition == v1.JobSucceeded || rj.Status.Condition == v1.JobStopped
}

func isRadixJobQueued(rj *v1.RadixJob) bool {
	return rj == nil || rj.Status.Condition == v1.JobQueued
}

func (job *Job) setStatusOfJob() error {
	pipelinejob, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(job.radixJob.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// No kubernetes job created yet, so nothing to sync
		return nil
	}

	if err != nil {
		return err
	}

	jobStatus := getJobConditionFromJobStatus(pipelinejob.Status)

	job.radixJob.Status.Condition = jobStatus
	job.radixJob.Status.Created = &job.radixJob.CreationTimestamp
	job.radixJob.Status.Started = pipelinejob.Status.StartTime

	if len(pipelinejob.Status.Conditions) > 0 {
		job.radixJob.Status.Ended = &pipelinejob.Status.Conditions[0].LastTransitionTime
	}

	steps, err := job.getJobSteps(pipelinejob)
	if err != nil {
		return err
	}

	environments, err := job.getJobEnvironments()
	if err != nil {
		return err
	}

	if len(environments) > 0 {
		job.radixJob.Status.TargetEnvs = environments
	}

	job.radixJob.Status.Steps = steps
	err = saveStatus(job.radixclient, job.radixJob, job.originalRadixJobStatus)

	if job.radixJob.Status.Condition == v1.JobSucceeded || job.radixJob.Status.Condition == v1.JobFailed {
		err = job.setNextJobToRunning()
	}

	return err
}

func (job *Job) stopJob() error {
	isRunning := job.radixJob.Status.Condition == v1.JobRunning

	if isRunning {
		// Delete pipeline job
		err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Delete(job.radixJob.Name, &metav1.DeleteOptions{})

		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		err = job.deleteStepJobs()
		if err != nil {
			return err
		}

		stoppedSteps := make([]v1.RadixJobStep, 0)
		for _, step := range job.radixJob.Status.Steps {
			if step.Condition != v1.JobSucceeded && step.Condition != v1.JobFailed {
				step.Condition = v1.JobStopped
			}

			stoppedSteps = append(stoppedSteps, step)
		}

		job.radixJob.Status.Steps = stoppedSteps
	}

	job.radixJob.Status.Created = &job.radixJob.CreationTimestamp
	job.radixJob.Status.Condition = v1.JobStopped
	job.radixJob.Status.Ended = &metav1.Time{Time: time.Now()}

	err := saveStatus(job.radixclient, job.radixJob, job.originalRadixJobStatus)
	if err == nil && isRunning {
		err = job.setNextJobToRunning()
	}

	return err
}

func (job *Job) deleteStepJobs() error {
	jobs, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s!=%s", kube.RadixJobNameLabel, job.radixJob.Name, kube.RadixJobTypeLabel, kube.RadixJobTypeJob),
	})

	if err != nil {
		return err
	}

	if len(jobs.Items) > 0 {
		for _, kubernetesJob := range jobs.Items {
			// Delete jobs
			err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Delete(kubernetesJob.Name, &metav1.DeleteOptions{})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (job *Job) setNextJobToRunning() error {
	rjList, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	rjs := sortJobsByActiveFromTimestampAsc(rjList.Items)
	for _, otherRj := range rjs {
		if otherRj.Name != job.radixJob.Name && otherRj.Status.Condition == v1.JobQueued {
			otherRj.Status.Condition = v1.JobRunning
			err = saveStatus(job.radixclient, &otherRj, v1.JobQueued) // previous status for this otherRj was Queued
			break
		}
	}

	return nil
}

func sortJobsByActiveFromTimestampAsc(rjs []v1.RadixJob) []v1.RadixJob {
	sort.Slice(rjs, func(i, j int) bool {
		return isRJ1ActiveAfterRJ2(&rjs[j], &rjs[i])
	})
	return rjs
}

func isRJ1ActiveAfterRJ2(rj1 *v1.RadixJob, rj2 *v1.RadixJob) bool {
	rj1ActiveFrom := rj1.CreationTimestamp
	rj2ActiveFrom := rj2.CreationTimestamp

	return rj2ActiveFrom.Before(&rj1ActiveFrom)
}

func (job *Job) queueJob() error {
	job.radixJob.Status.Created = &job.radixJob.CreationTimestamp
	job.radixJob.Status.Condition = v1.JobQueued
	return saveStatus(job.radixclient, job.radixJob, job.originalRadixJobStatus)
}

func (job *Job) getJobSteps(kubernetesJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	steps := []v1.RadixJobStep{}

	pipelinePod, err := job.getPipelinePod()
	if err != nil {
		return nil, err
	} else if pipelinePod == nil {
		return steps, nil
	}

	if len(pipelinePod.Status.ContainerStatuses) == 0 {
		return steps, nil
	}

	pipelineType := job.radixJob.Spec.PipeLineType

	switch pipelineType {
	case v1.Build, v1.BuildDeploy:
		return job.getJobStepsBuildPipeline(pipelinePod, kubernetesJob)
	case v1.Promote:
		return job.getJobStepsPromotePipeline(pipelinePod, kubernetesJob)
	// TODO
	case v1.Deploy:
		return job.getJobStepsBuildPipeline(pipelinePod, kubernetesJob)
	}

	return steps, nil
}

func (job *Job) getJobStepsBuildPipeline(pipelinePod *corev1.Pod, kubernetesJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	steps := []v1.RadixJobStep{}

	pipelineJobStep := getPipelineJobStep(pipelinePod)

	// Clone of radix config should be represented
	cloneConfigStep, applyConfigStep := job.getCloneConfigStep()
	jobStepsLabelSelector := fmt.Sprintf("%s=%s, %s!=%s", kube.RadixImageTagLabel, kubernetesJob.Labels[kube.RadixImageTagLabel], kube.RadixJobTypeLabel, kube.RadixJobTypeJob)

	jobStepList, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: jobStepsLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	// pipeline coordinator
	steps = append(steps, cloneConfigStep, applyConfigStep, pipelineJobStep)
	for _, jobStep := range jobStepList.Items {
		if strings.EqualFold(jobStep.GetLabels()[kube.RadixJobTypeLabel], kube.RadixJobTypeCloneConfig) {
			// Clone of radix config represented as an initial step
			continue
		}

		// Does it hold annotations component to container mapping
		componentImagesAnnotation := jobStep.GetObjectMeta().GetAnnotations()[kube.RadixComponentImagesAnnotation]
		componentImages := make(map[string]pipeline.ComponentImage)

		if !strings.EqualFold(componentImagesAnnotation, "") {
			err := json.Unmarshal([]byte(componentImagesAnnotation), &componentImages)

			if err != nil {
				return nil, err
			}
		}

		jobStepPod, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "job-name", jobStep.Name),
		})

		if err != nil {
			return nil, err
		}

		if len(jobStepPod.Items) == 0 {
			continue
		}

		pod := jobStepPod.Items[0]
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			if strings.HasPrefix(containerStatus.Name, git.InternalContainerPrefix) {
				continue
			}

			steps = append(steps, getJobStepWithNoComponents(pod.GetName(), &containerStatus))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			components := getComponentsForContainer(containerStatus.Name, componentImages)
			steps = append(steps, getJobStep(pod.GetName(), &containerStatus, components))
		}
	}

	return steps, nil
}

func getComponentsForContainer(name string, componentImages map[string]pipeline.ComponentImage) []string {
	components := make([]string, 0)

	for component, componentImage := range componentImages {
		if strings.EqualFold(componentImage.ContainerName, name) {
			components = append(components, component)
		}
	}

	return components
}

func (job *Job) getJobStepsPromotePipeline(pipelinePod *corev1.Pod, kubernetesJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	steps := []v1.RadixJobStep{}
	pipelineJobStep := getJobStepWithNoComponents(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0])
	steps = append(steps, pipelineJobStep)
	return steps, nil
}

func (job *Job) getPipelinePod() (*corev1.Pod, error) {
	pods, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(metav1.ListOptions{
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

func (job *Job) getCloneConfigStep() (v1.RadixJobStep, v1.RadixJobStep) {
	labelSelector := fmt.Sprintf("%s=%s, %s=%s", kube.RadixJobNameLabel, job.radixJob.GetName(), kube.RadixJobTypeLabel, kube.RadixJobTypeCloneConfig)

	cloneConfigStep, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil || len(cloneConfigStep.Items) != 1 {
		return v1.RadixJobStep{}, v1.RadixJobStep{}
	}

	cloneConfigPod, err := job.kubeclient.CoreV1().Pods(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "job-name", cloneConfigStep.Items[0].Name),
	})

	if err != nil || len(cloneConfigPod.Items) != 1 {
		return v1.RadixJobStep{}, v1.RadixJobStep{}
	}
	applyConfigPod := cloneConfigPod.Items[0]

	cloneContainerStatus := getContainerStatusByName(git.CloneConfigContainerName, applyConfigPod.Status.InitContainerStatuses)
	if cloneContainerStatus == nil {
		return v1.RadixJobStep{}, v1.RadixJobStep{}
	}
	cloneContainerStep := getJobStepWithNoComponents(cloneConfigPod.Items[0].GetName(), cloneContainerStatus)
	applyContainerJobStep := getJobStepWithNoComponents(applyConfigPod.GetName(), &applyConfigPod.Status.ContainerStatuses[0])

	return cloneContainerStep, applyContainerJobStep
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
	var startedAt metav1.Time
	var finishedAt metav1.Time

	status := v1.JobSucceeded

	if containerStatus == nil {
		status = v1.JobWaiting

	} else if containerStatus.State.Terminated != nil {
		startedAt = containerStatus.State.Terminated.StartedAt
		finishedAt = containerStatus.State.Terminated.FinishedAt

		if containerStatus.State.Terminated.ExitCode > 0 {
			status = v1.JobFailed
		}

	} else if containerStatus.State.Running != nil {
		startedAt = containerStatus.State.Running.StartedAt
		status = v1.JobRunning

	} else if containerStatus.State.Waiting != nil {
		status = v1.JobWaiting

	}

	return v1.RadixJobStep{
		Name:       containerName,
		Started:    &startedAt,
		Ended:      &finishedAt,
		Condition:  status,
		PodName:    podName,
		Components: components,
	}
}

func (job *Job) getJobEnvironments() ([]string, error) {
	deploymentsLinkedToJob, err := job.radixclient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(metav1.ListOptions{
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

func saveStatus(radixClient radixclient.Interface, rj *v1.RadixJob, originalRadixJobStatus v1.RadixJobCondition) error {
	_, err := radixClient.RadixV1().RadixJobs(rj.GetNamespace()).UpdateStatus(rj)
	if err == nil && originalRadixJobStatus != rj.Status.Condition {
		metrics.RadixJobStatusChanged(rj)
	}
	return err
}

func (job *Job) maintainHistoryLimit() {
	historyLimit := os.Getenv(defaults.JobsHistoryLimitEnvironmentVariable)
	if historyLimit != "" {
		limit, err := strconv.Atoi(historyLimit)
		if err != nil {
			log.Warnf("'%s' is not set to a proper number, %s, and cannot be parsed.", defaults.JobsHistoryLimitEnvironmentVariable, historyLimit)
			return
		}

		allRJs, err := job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).List(metav1.ListOptions{})
		if err != nil {
			log.Errorf("Failed to get all RadixDeployments. Error was %v", err)
			return
		}

		if len(allRJs.Items) > limit {
			jobs := allRJs.Items
			numToDelete := len(jobs) - limit
			if numToDelete <= 0 {
				return
			}

			jobs = sortJobsByActiveFromTimestampAsc(jobs)
			for i := 0; i < numToDelete; i++ {
				log.Infof("Removing job %s from %s", jobs[i].Name, jobs[i].Namespace)
				job.radixclient.RadixV1().RadixJobs(job.radixJob.Namespace).Delete(jobs[i].Name, &metav1.DeleteOptions{})
			}
		}
	}
}
