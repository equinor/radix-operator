package job

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
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
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	radixJob    *v1.RadixJob
}

// NewJob Constructor
func NewJob(kubeclient kubernetes.Interface, radixclient radixclient.Interface, radixJob *v1.RadixJob) Job {
	kubeutil, _ := kube.New(kubeclient)

	return Job{
		kubeclient,
		radixclient,
		kubeutil, radixJob}
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (job *Job) OnSync() error {
	job.restoreStatus()

	if IsRadixJobDone(job.radixJob) {
		log.Warnf("Ignoring RadixJob %s/%s as it's no longer active.", job.radixJob.Namespace, job.radixJob.Name)
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
		return job.createJob()
	} else if err != nil {
		return err
	}

	return nil
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
			err = saveStatus(job.radixclient, job.radixJob)
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

	job.radixJob.Status.Steps = steps
	job.radixJob.Status.TargetEnvs = environments
	err = saveStatus(job.radixclient, job.radixJob)

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

	err := saveStatus(job.radixclient, job.radixJob)
	if err == nil && isRunning {
		err = job.setNextJobToRunning()
	}

	return err
}

func (job *Job) deleteStepJobs() error {
	jobs, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s, %s!=%s", kube.RadixJobNameLabel, job.radixJob.Name, kube.RadixJobTypeLabel, RadixJobTypeJob),
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
			err = saveStatus(job.radixclient, &otherRj)
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
	return saveStatus(job.radixclient, job.radixJob)
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
	}

	return steps, nil
}

func (job *Job) getJobStepsBuildPipeline(pipelinePod *corev1.Pod, kubernetesJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	steps := []v1.RadixJobStep{}
	if len(pipelinePod.Status.InitContainerStatuses) == 0 {
		return steps, nil
	}

	pipelineJobStep := getPipelineJobStep(pipelinePod)
	cloneContainerStatus := getCloneConfigContainerStatus(pipelinePod)
	if cloneContainerStatus == nil {
		return steps, nil
	}

	// Clone of radix config should be represented
	pipelineCloneStep := getJobStep(pipelinePod.GetName(), cloneContainerStatus)
	jobStepsLabelSelector := fmt.Sprintf("%s=%s, %s!=%s", kube.RadixImageTagLabel, kubernetesJob.Labels[kube.RadixImageTagLabel], kube.RadixJobTypeLabel, RadixJobTypeJob)

	jobStepList, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).List(metav1.ListOptions{
		LabelSelector: jobStepsLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	// pipeline coordinator
	steps = append(steps, pipelineCloneStep, pipelineJobStep)
	for _, jobStep := range jobStepList.Items {
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

			steps = append(steps, getJobStep(pod.GetName(), &containerStatus))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			steps = append(steps, getJobStep(pod.GetName(), &containerStatus))
		}
	}

	return steps, nil
}

func (job *Job) getJobStepsPromotePipeline(pipelinePod *corev1.Pod, kubernetesJob *batchv1.Job) ([]v1.RadixJobStep, error) {
	steps := []v1.RadixJobStep{}
	pipelineJobStep := getJobStep(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0])
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
	var pipelineJobStep v1.RadixJobStep

	cloneContainerStatus := getCloneConfigContainerStatus(pipelinePod)
	if cloneContainerStatus == nil {
		return v1.RadixJobStep{}
	}

	if cloneContainerStatus.State.Terminated != nil &&
		cloneContainerStatus.State.Terminated.ExitCode > 0 {
		pipelineJobStep = getJobStepWithContainerName(pipelinePod.GetName(),
			pipelinePod.Status.ContainerStatuses[0].Name, cloneContainerStatus)
	} else {
		pipelineJobStep = getJobStep(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0])
	}

	return pipelineJobStep
}

func getCloneConfigContainerStatus(pipelinePod *corev1.Pod) *corev1.ContainerStatus {
	for _, containerStatus := range pipelinePod.Status.InitContainerStatuses {
		if containerStatus.Name == git.CloneConfigContainerName {
			return &containerStatus
		}
	}

	return nil
}

func getJobStep(podName string, containerStatus *corev1.ContainerStatus) v1.RadixJobStep {
	return getJobStepWithContainerName(podName, containerStatus.Name, containerStatus)
}

func getJobStepWithContainerName(podName, containerName string, containerStatus *corev1.ContainerStatus) v1.RadixJobStep {
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
		Name:      containerName,
		Started:   &startedAt,
		Ended:     &finishedAt,
		Condition: status,
		PodName:   podName,
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

func saveStatus(radixClient radixclient.Interface, rj *v1.RadixJob) error {
	_, err := radixClient.RadixV1().RadixJobs(rj.GetNamespace()).UpdateStatus(rj)
	return err
}
