package batch

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (s *syncer) syncStatus(ctx context.Context, reconcileError error) error {
	jobStatuses, err := s.buildJobStatuses(ctx)
	if err != nil {
		return err
	}

	conditionType := radixv1.BatchConditionTypeActive
	switch {
	case slice.All(jobStatuses, isJobStatusWaiting):
		conditionType = radixv1.BatchConditionTypeWaiting
	case slice.All(jobStatuses, isJobStatusDone):
		conditionType = radixv1.BatchConditionTypeCompleted
	}

	err = s.updateStatus(ctx, func(currStatus *radixv1.RadixBatchStatus) {
		currStatus.JobStatuses = jobStatuses
		currStatus.Condition.Type = conditionType
		currStatus.Condition.Reason = ""
		currStatus.Condition.Message = ""

		switch conditionType {
		case radixv1.BatchConditionTypeWaiting:
			currStatus.Condition.ActiveTime = nil
			currStatus.Condition.CompletionTime = nil
		case radixv1.BatchConditionTypeActive:
			now := metav1.Now()
			if currStatus.Condition.ActiveTime == nil {
				currStatus.Condition.ActiveTime = &now
			}
			currStatus.Condition.CompletionTime = nil
		case radixv1.BatchConditionTypeCompleted:
			now := metav1.Now()
			if currStatus.Condition.ActiveTime == nil {
				currStatus.Condition.ActiveTime = &now
			}
			if currStatus.Condition.CompletionTime == nil {
				currStatus.Condition.CompletionTime = &now
			}
		}
	})
	if err != nil {
		return err
	}

	if status := reconcileStatus(nil); stdErrors.As(reconcileError, &status) {
		// Do not return an error if reconcileError indicates
		// invalid RadixDeployment reference as long as all jobs are in a done state
		if status.Status().Reason == invalidDeploymentReferenceReason && slice.All(jobStatuses, isJobStatusDone) {
			return nil
		}

		if err := s.updateStatus(ctx, func(currStatus *radixv1.RadixBatchStatus) { currStatus.Condition = status.Status() }); err != nil {
			return err
		}
	}

	return reconcileError
}

func (s *syncer) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixBatchStatus)) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixBatch, err := s.radixClient.RadixV1().RadixBatches(s.radixBatch.GetNamespace()).Get(ctx, s.radixBatch.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&radixBatch.Status)
		updatedRadixBatch, err := s.radixClient.
			RadixV1().
			RadixBatches(radixBatch.GetNamespace()).
			UpdateStatus(ctx, radixBatch, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.radixBatch = updatedRadixBatch
		return nil
	})
	return err
}

func isJobStatusCondition(conditionType batchv1.JobConditionType) func(batchv1.JobCondition) bool {
	return func(condition batchv1.JobCondition) bool {
		return condition.Type == conditionType && condition.Status == corev1.ConditionTrue
	}
}

func (s *syncer) buildJobStatuses(ctx context.Context) ([]radixv1.RadixBatchJobStatus, error) {
	var jobStatuses []radixv1.RadixBatchJobStatus

	jobs, err := s.kubeUtil.ListJobsWithSelector(ctx, s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return nil, err
	}

	for _, batchJob := range s.radixBatch.Spec.Jobs {
		jobStatus := s.buildBatchJobStatus(ctx, &batchJob, jobs)
		jobStatuses = append(jobStatuses, jobStatus)
	}

	return jobStatuses, nil
}

func (s *syncer) buildBatchJobStatus(ctx context.Context, batchJob *radixv1.RadixBatchJob, allJobs []*batchv1.Job) radixv1.RadixBatchJobStatus {
	restartedJob, isRestartedJob := s.restartedJobs[batchJob.Name]

	currentStatus, hasCurrentStatus := slice.FindFirst(s.radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == batchJob.Name
	})

	if !isRestartedJob && hasCurrentStatus && isJobStatusDone(currentStatus) {
		return currentStatus
	}

	status := radixv1.RadixBatchJobStatus{
		Name:  batchJob.Name,
		Phase: radixv1.BatchJobPhaseWaiting,
	}

	if isRestartedJob {
		status.Restart = restartedJob.Restart
	}

	if hasCurrentStatus && !isRestartedJob {
		status.Phase = currentStatus.Phase
	}

	if isBatchJobStopRequested(batchJob) {
		status.Phase = radixv1.BatchJobPhaseStopped
		now := metav1.Now()
		status.EndTime = &now
		if hasCurrentStatus {
			status.CreationTime = currentStatus.CreationTime
			status.StartTime = currentStatus.StartTime
			status.Message = currentStatus.Message
			status.Reason = currentStatus.Reason
		}
		s.updateJobAndPodStatuses(ctx, batchJob.Name, &status)
		return status
	}

	job, jobFound := slice.FindFirst(allJobs, isKubeJobForBatchJob(batchJob))
	if !jobFound {
		return status
	}
	jobBackoffLimit := getJobBackoffLimit(job)
	status.CreationTime = &job.CreationTimestamp
	status.Failed = job.Status.Failed

	var uncountedSucceeded, uncountedFailed int
	if uncounted := job.Status.UncountedTerminatedPods; uncounted != nil {
		uncountedSucceeded, uncountedFailed = len(uncounted.Succeeded), len(uncounted.Failed)
	}
	jobConditionsSortedDesc := getJobConditionsSortedDesc(job)
	if (job.Status.Succeeded+int32(uncountedSucceeded)) > 0 &&
		s.setJobStatus(ctx, batchJob, &status, job, jobConditionsSortedDesc, radixv1.BatchJobPhaseSucceeded, batchv1.JobComplete) {
		return status
	}
	if (job.Status.Failed+int32(uncountedFailed)) == jobBackoffLimit+1 &&
		s.setJobStatus(ctx, batchJob, &status, job, jobConditionsSortedDesc, radixv1.BatchJobPhaseFailed, batchv1.JobFailed) {
		return status
	}
	if job.Status.Active > 0 {
		status.Phase = radixv1.BatchJobPhaseActive
		status.StartTime = job.Status.StartTime
		if job.Status.Ready != nil && job.Status.Active == *job.Status.Ready {
			status.Phase = radixv1.BatchJobPhaseRunning
		}
	}
	if len(jobConditionsSortedDesc) > 0 {
		status.Reason = jobConditionsSortedDesc[0].Reason
		status.Message = jobConditionsSortedDesc[0].Message
	}
	s.updateJobAndPodStatuses(ctx, batchJob.Name, &status)
	return status
}

func (s *syncer) setJobStatus(ctx context.Context, batchJob *radixv1.RadixBatchJob, status *radixv1.RadixBatchJobStatus, job *batchv1.Job, jobConditionsSortedDesc []batchv1.JobCondition, phase radixv1.RadixBatchJobPhase, conditionType batchv1.JobConditionType) bool {
	if condition, ok := slice.FindFirst(jobConditionsSortedDesc, isJobStatusCondition(conditionType)); ok {
		status.Phase = phase
		status.StartTime = job.Status.StartTime
		status.Reason = condition.Reason
		status.Message = condition.Message
		status.EndTime = pointers.Ptr(condition.LastTransitionTime)
		s.updateJobAndPodStatuses(ctx, batchJob.Name, status)
		return true
	}
	return false
}

func getJobBackoffLimit(job *batchv1.Job) int32 {
	if job.Spec.BackoffLimit != nil {
		return *job.Spec.BackoffLimit
	}
	return 0
}

func (s *syncer) updateJobAndPodStatuses(ctx context.Context, batchJobName string, jobStatus *radixv1.RadixBatchJobStatus) {
	jobComponentName := s.radixBatch.GetLabels()[kube.RadixComponentLabel]
	podStatusMap := getPodStatusMap(jobStatus)
	for _, pod := range s.getJobPods(ctx, batchJobName) {
		podStatus := getOrCreatePodStatusForPod(&pod, jobStatus, podStatusMap)
		if containerStatus, ok := s.getJobComponentContainerStatus(jobComponentName, pod); ok {
			setPodStatusByPodLastContainerStatus(containerStatus, podStatus, jobStatus)
			continue
		}
		setPodStatusByPodCondition(&pod, podStatus, jobStatus)
	}
}

func (s *syncer) getJobComponentContainerStatus(containerName string, pod corev1.Pod) (*corev1.ContainerStatus, bool) {
	if containerStatus, ok := slice.FindFirst(pod.Status.ContainerStatuses, func(containerStatus corev1.ContainerStatus) bool {
		return containerStatus.Name == containerName
	}); ok {
		return &containerStatus, true
	}
	return nil, false
}

func getOrCreatePodStatusForPod(pod *corev1.Pod, jobStatus *radixv1.RadixBatchJobStatus, podStatusMap map[string]*radixv1.RadixBatchJobPodStatus) *radixv1.RadixBatchJobPodStatus {
	podStatus, ok := podStatusMap[pod.GetName()]
	if !ok {
		jobStatus.RadixBatchJobPodStatuses = append(jobStatus.RadixBatchJobPodStatuses, radixv1.RadixBatchJobPodStatus{
			Name:         pod.GetName(),
			Phase:        radixv1.RadixBatchJobPodPhase(pod.Status.Phase),
			CreationTime: pointers.Ptr(pod.GetCreationTimestamp()),
			PodIndex:     len(jobStatus.RadixBatchJobPodStatuses),
		})
		podStatus = &jobStatus.RadixBatchJobPodStatuses[len(jobStatus.RadixBatchJobPodStatuses)-1]
	}
	if podStatePhaseShouldBeStopped(jobStatus, podStatus) {
		podStatus.Phase = radixv1.PodStopped
	}
	return podStatus
}

func (s *syncer) getJobPods(ctx context.Context, batchJobName string) []corev1.Pod {
	jobPods, err := s.kubeUtil.KubeClient().CoreV1().Pods(s.radixBatch.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: s.batchJobIdentifierLabel(batchJobName, s.radixBatch.GetLabels()[kube.RadixAppLabel]).String()})
	if err != nil || len(jobPods.Items) == 0 {
		return nil
	}
	return jobPods.Items
}

func setPodStatusByPodLastContainerStatus(containerStatus *corev1.ContainerStatus, podStatus *radixv1.RadixBatchJobPodStatus, jobStatus *radixv1.RadixBatchJobStatus) {
	podStatus.RestartCount = containerStatus.RestartCount
	podStatus.Image = containerStatus.Image
	podStatus.ImageID = containerStatus.ImageID
	switch {
	case containerStatus.State.Running != nil:
		podStatus.Phase = radixv1.PodRunning
		podStatus.StartTime = &containerStatus.State.Running.StartedAt
	case containerStatus.State.Waiting != nil:
		podStatus.Phase = radixv1.PodPending
		podStatus.Message = containerStatus.State.Waiting.Message
		podStatus.Reason = containerStatus.State.Waiting.Reason
	case containerStatus.State.Terminated != nil:
		podStatus.Message = containerStatus.State.Terminated.Message
		podStatus.Reason = containerStatus.State.Terminated.Reason
		if strings.EqualFold(containerStatus.State.Terminated.Reason, "Completed") &&
			containerStatus.State.Terminated.ExitCode == 0 {
			podStatus.Phase = radixv1.PodSucceeded
			podStatus.ExitCode = 0
			podStatus.StartTime = &containerStatus.State.Terminated.StartedAt
			podStatus.EndTime = &containerStatus.State.Terminated.FinishedAt
			return
		}
		podStatus.Phase = radixv1.PodFailed
		podStatus.Reason = containerStatus.State.Terminated.Reason
		podStatus.ExitCode = containerStatus.State.Terminated.ExitCode
		podStatus.StartTime = &containerStatus.State.Terminated.StartedAt
		podStatus.EndTime = &containerStatus.State.Terminated.FinishedAt
		podStatus.Message = extendMessage(podStatus)
	}
	if podStatePhaseShouldBeStopped(jobStatus, podStatus) {
		podStatus.Phase = radixv1.PodStopped
	}
}

func extendMessage(podStatus *radixv1.RadixBatchJobPodStatus) string {
	var messageItems []string
	if podStatus.Reason == "OOMKilled" {
		messageItems = append(messageItems, "Out of memory.")
	}
	if len(podStatus.Message) > 0 {
		messageItems = append(messageItems, podStatus.Message)
	}
	if len(messageItems) > 0 {
		return strings.Join(messageItems, "\n")
	}
	return ""
}

func setPodStatusByPodCondition(pod *corev1.Pod, podStatus *radixv1.RadixBatchJobPodStatus, jobStatus *radixv1.RadixBatchJobStatus) {
	if len(pod.Status.Conditions) == 0 {
		return
	}
	conditions := pod.Status.Conditions
	sort.Slice(conditions, func(i, j int) bool {
		if conditions[i].LastTransitionTime.Time == conditions[j].LastTransitionTime.Time {
			return i < j
		}
		return conditions[i].LastTransitionTime.After(conditions[j].LastTransitionTime.Time)
	})
	podStatus.Reason = conditions[0].Reason
	podStatus.Message = conditions[0].Message
	podStatus.Phase = radixv1.RadixBatchJobPodPhase(pod.Status.Phase)
	if podStatePhaseShouldBeStopped(jobStatus, podStatus) {
		podStatus.Phase = radixv1.PodStopped
	}
}

func podStatePhaseShouldBeStopped(jobStatus *radixv1.RadixBatchJobStatus, podStatus *radixv1.RadixBatchJobPodStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseStopped &&
		(podStatus.Phase == radixv1.PodPending || podStatus.Phase == radixv1.PodRunning)
}

func getPodStatusMap(status *radixv1.RadixBatchJobStatus) map[string]*radixv1.RadixBatchJobPodStatus {
	podStatusMap := make(map[string]*radixv1.RadixBatchJobPodStatus, len(status.RadixBatchJobPodStatuses))
	for i := 0; i < len(status.RadixBatchJobPodStatuses); i++ {
		podStatusMap[status.RadixBatchJobPodStatuses[i].Name] = &status.RadixBatchJobPodStatuses[i]
	}
	return podStatusMap
}

func getJobConditionsSortedDesc(job *batchv1.Job) []batchv1.JobCondition {
	descSortedJobConditions := job.Status.Conditions
	sort.Slice(descSortedJobConditions, func(i, j int) bool {
		return descSortedJobConditions[i].LastTransitionTime.After(descSortedJobConditions[j].LastTransitionTime.Time)
	})
	return descSortedJobConditions
}

func (s *syncer) restoreStatus(ctx context.Context) error {
	if restoredStatus, ok := s.radixBatch.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.radixBatch.Status).IsZero() {
			var status radixv1.RadixBatchStatus

			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				return fmt.Errorf("unable to restore status for batch %s.%s from annotation: %w", s.radixBatch.GetNamespace(), s.radixBatch.GetName(), err)
			}

			return s.updateStatus(ctx, func(currStatus *radixv1.RadixBatchStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}
