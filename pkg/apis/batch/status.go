package batch

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isJobStatusWaiting(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseWaiting
}

func isJobStatusDone(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded ||
		jobStatus.Phase == radixv1.BatchJobPhaseFailed ||
		jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func (s *syncer) syncStatus(reconcileError error) error {
	jobStatuses, err := s.buildJobStatuses()
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

	err = s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) {
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

		if err := s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) { currStatus.Condition = status.Status() }); err != nil {
			return err
		}
	}

	return reconcileError
}

func (s *syncer) updateStatus(changeStatusFunc func(currStatus *radixv1.RadixBatchStatus)) error {
	changeStatusFunc(&s.radixBatch.Status)
	updatedRadixBatch, err := s.radixClient.
		RadixV1().
		RadixBatches(s.radixBatch.GetNamespace()).
		UpdateStatus(context.TODO(), s.radixBatch, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.radixBatch = updatedRadixBatch
	return nil
}

func isJobStatusCondition(conditionType batchv1.JobConditionType) func(batchv1.JobCondition) bool {
	return func(condition batchv1.JobCondition) bool {
		return condition.Type == conditionType && condition.Status == corev1.ConditionTrue
	}
}

func (s *syncer) buildJobStatuses() ([]radixv1.RadixBatchJobStatus, error) {
	var jobStatuses []radixv1.RadixBatchJobStatus

	jobs, err := s.kubeUtil.ListJobsWithSelector(s.radixBatch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return nil, err
	}

	for _, batchJob := range s.radixBatch.Spec.Jobs {
		jobStatus := s.buildBatchJobStatus(&batchJob, jobs)
		jobStatuses = append(jobStatuses, jobStatus)
	}

	return jobStatuses, nil
}

func (s *syncer) buildBatchJobStatus(batchJob *radixv1.RadixBatchJob, allJobs []*batchv1.Job) radixv1.RadixBatchJobStatus {
	currentStatus := slice.FindAll(s.radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == batchJob.Name
	})
	if len(currentStatus) > 0 && isBatchJobPhaseDone(currentStatus[0].Phase) {
		return currentStatus[0]
	}

	status := radixv1.RadixBatchJobStatus{
		Name:  batchJob.Name,
		Phase: radixv1.BatchJobPhaseWaiting,
	}
	if len(currentStatus) > 0 {
		status.Restart = currentStatus[0].Restart
	}

	if isBatchJobStopRequested(batchJob) {
		now := metav1.Now()
		status.Phase = radixv1.BatchJobPhaseStopped
		status.Message = currentStatus[0].Message
		status.Reason = currentStatus[0].Reason
		status.EndTime = &now
		if len(currentStatus) > 0 {
			status.CreationTime = currentStatus[0].CreationTime
			status.StartTime = currentStatus[0].StartTime
		}
		s.updateStatusByJobPods(batchJob.Name, &status)
		return status
	}

	job, jobFound := slice.FindFirst(allJobs, func(job *batchv1.Job) bool { return isResourceLabeledWithBatchJobName(batchJob.Name, job) })
	if !jobFound {
		return status
	}
	status.CreationTime = &job.CreationTimestamp
	status.Failed = job.Status.Failed

	jobConditionsSortedDesc := getJobConditionsSortedDesc(job)
	if condition, ok := slice.FindFirst(jobConditionsSortedDesc, isJobStatusCondition(batchv1.JobComplete)); ok {
		status.Phase = radixv1.BatchJobPhaseSucceeded
		status.StartTime = job.Status.StartTime
		status.EndTime = job.Status.CompletionTime
		status.Message = condition.Message
		status.Reason = condition.Reason
		s.updateStatusByJobPods(batchJob.Name, &status)
		return status
	}
	if failedCondition, ok := slice.FindFirst(jobConditionsSortedDesc, isJobStatusCondition(batchv1.JobFailed)); ok {
		status.Phase = radixv1.BatchJobPhaseFailed
		status.StartTime = job.Status.StartTime
		status.EndTime = &failedCondition.LastTransitionTime
		status.Reason = failedCondition.Reason
		status.Message = failedCondition.Message
		s.updateStatusByJobPods(batchJob.Name, &status)
		return status
	}
	if job.Status.Active > 0 {
		status.Phase = radixv1.BatchJobPhaseActive
		status.StartTime = job.Status.StartTime
	}
	if len(jobConditionsSortedDesc) > 0 {
		status.Reason = jobConditionsSortedDesc[0].Reason
		status.Message = jobConditionsSortedDesc[0].Message
	}
	s.updateStatusByJobPods(batchJob.Name, &status)
	return status
}

func (s *syncer) updateStatusByJobPods(batchJobName string, status *radixv1.RadixBatchJobStatus) {
	jobPods, err := s.kubeUtil.KubeClient().CoreV1().Pods(s.radixBatch.GetNamespace()).List(context.Background(), metav1.ListOptions{
		LabelSelector: s.batchJobIdentifierLabel(batchJobName, s.radixBatch.GetLabels()[kube.RadixAppLabel]).String()})
	if err != nil || len(jobPods.Items) == 0 {
		return
	}
	pod := jobPods.Items[0]
	if pod.Status.Phase == corev1.PodSucceeded {
		return
	}
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		status.Phase = radixv1.BatchJobPhaseFailed
		status.Message = pod.Status.ContainerStatuses[0].State.Terminated.Message
		status.Reason = pod.Status.ContainerStatuses[0].State.Terminated.Reason
	}
}

func getJobConditionsSortedDesc(job *batchv1.Job) []batchv1.JobCondition {
	descSortedJobConditions := job.Status.Conditions
	sort.Slice(descSortedJobConditions, func(i, j int) bool {
		return descSortedJobConditions[i].LastTransitionTime.After(descSortedJobConditions[j].LastTransitionTime.Time)
	})
	return descSortedJobConditions
}

func (s *syncer) restoreStatus() error {
	if restoredStatus, ok := s.radixBatch.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.radixBatch.Status).IsZero() {
			var status radixv1.RadixBatchStatus

			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				return fmt.Errorf("unable to restore status for batch %s.%s from annotation: %w", s.radixBatch.GetNamespace(), s.radixBatch.GetName(), err)
			}

			return s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}
