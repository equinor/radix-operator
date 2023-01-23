package batch

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isJobStatusRunning(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseRunning
}

func isJobStatusDone(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded ||
		jobStatus.Phase == radixv1.BatchJobPhaseFailed ||
		jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func (s *syncer) syncStatus(reconcileError error) error {
	if status := reconcileStatus(nil); errors.As(reconcileError, &status) {
		if err := s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) { currStatus.Condition = status.Status() }); err != nil {
			return err
		}
		return reconcileError
	}

	jobStatuses, err := s.buildJobStatuses()
	if err != nil {
		return err
	}

	conditionType := radixv1.BatchConditionTypeWaiting
	switch {
	case slice.Any(jobStatuses, isJobStatusRunning):
		conditionType = radixv1.BatchConditionTypeRunning
	case slice.All(jobStatuses, isJobStatusDone):
		conditionType = radixv1.BatchConditionTypeCompleted
	}

	status := radixv1.RadixBatchStatus{
		Condition:   radixv1.RadixBatchCondition{Type: conditionType},
		JobStatuses: jobStatuses,
	}

	if err := s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) { *currStatus = status }); err != nil {
		return err
	}

	return reconcileError
}

func (s *syncer) updateStatus(changeStatusFunc func(currStatus *radixv1.RadixBatchStatus)) error {
	changeStatusFunc(&s.batch.Status)
	updatedJob, err := s.radixclient.
		RadixV1().
		RadixBatches(s.batch.GetNamespace()).
		UpdateStatus(context.TODO(), s.batch, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.batch = updatedJob
	return nil
}

func isJobStatusCondition(conditionType batchv1.JobConditionType) func(batchv1.JobCondition) bool {
	return func(condition batchv1.JobCondition) bool {
		return condition.Type == conditionType && condition.Status == corev1.ConditionTrue
	}
}

func (s *syncer) buildJobStatuses() ([]radixv1.RadixBatchJobStatus, error) {
	var jobStatuses []radixv1.RadixBatchJobStatus

	jobs, err := s.kubeutil.ListJobsWithSelector(s.batch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return nil, err
	}

	pods, err := s.kubeclient.CoreV1().Pods(s.batch.GetNamespace()).List(context.Background(), metav1.ListOptions{LabelSelector: s.batchIdentifierLabel().String()})
	if err != nil {
		return nil, err
	}

	for _, batchJob := range s.batch.Spec.Jobs {
		jobStatuses = append(jobStatuses, s.buildBatchJobStatus(batchJob, jobs, pods.Items))
	}

	return jobStatuses, nil
}

func (s *syncer) buildBatchJobStatus(batchJob radixv1.RadixBatchJob, allJobs []*batchv1.Job, allPods []corev1.Pod) radixv1.RadixBatchJobStatus {
	currentStatus := slice.FindAll(s.batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == batchJob.Name
	})
	if len(currentStatus) > 0 && isBatchJobPhaseDone(currentStatus[0].Phase) {
		return currentStatus[0]
	}

	status := radixv1.RadixBatchJobStatus{
		Name:  batchJob.Name,
		Phase: radixv1.BatchJobPhaseWaiting,
	}

	if isBatchJobStopRequested(batchJob) {
		status.Phase = radixv1.BatchJobPhaseStopped
		return status
	}

	if jobs := slice.FindAll(allJobs, func(job *batchv1.Job) bool { return isResourceLabeledWithBatchJobName(batchJob.Name, job) }); len(jobs) > 0 {
		job := jobs[0]
		status.Created = &job.CreationTimestamp

		switch {
		case job.Status.Active > 0:
			status.Phase = radixv1.BatchJobPhaseRunning
		case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobComplete)):
			status.Phase = radixv1.BatchJobPhaseSucceeded
		case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobFailed)):
			status.Phase = radixv1.BatchJobPhaseFailed
		}

		switch status.Phase {
		case radixv1.BatchJobPhaseRunning:
			status.Started = job.Status.StartTime
		case radixv1.BatchJobPhaseSucceeded:
			status.Started = job.Status.StartTime
			status.Ended = job.Status.CompletionTime
		case radixv1.BatchJobPhaseFailed:
			status.Started = job.Status.StartTime
		}

		if status.Phase == radixv1.BatchJobPhaseFailed || status.Phase == radixv1.BatchJobPhaseWaiting {
			pods := slice.FindAll(allPods, func(pod corev1.Pod) bool { return isResourceLabeledWithBatchJobName(batchJob.Name, &pod) })
			status.Reason, status.Message = getReasonAndMessageFromPendingOrFailedPod(pods)
		}
	}

	return status
}

func (s *syncer) restoreStatus() error {
	if restoredStatus, ok := s.batch.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.batch.Status).IsZero() {
			var status radixv1.RadixBatchStatus

			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				log.Warnf("unable to restore status for batch %s.%s from annotation", s.batch.GetNamespace(), s.batch.GetName())
				return nil
			}

			return s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}

func getReasonAndMessageFromPendingOrFailedPod(pods []corev1.Pod) (reason, message string) {
	if len(pods) == 0 {
		return
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp.Time) })
	pod := pods[0]

	switch pod.Status.Phase {
	case corev1.PodPending, corev1.PodUnknown:
		reason = pod.Status.Reason
		message = pod.Status.Message

		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Waiting != nil {
			reason = pod.Status.ContainerStatuses[0].State.Waiting.Reason
			message = pod.Status.ContainerStatuses[0].State.Waiting.Message
		}
	case corev1.PodFailed:
		reason = pod.Status.Reason
		message = pod.Status.Message

		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
			if len(reason) == 0 {
				reason = pod.Status.ContainerStatuses[0].State.Terminated.Reason
			}

			if len(message) == 0 {
				message = pod.Status.ContainerStatuses[0].State.Terminated.Message
			}
		}
	}

	return
}
