package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

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

	for _, batchJob := range s.batch.Spec.Jobs {
		jobStatuses = append(jobStatuses, s.buildBatchJobStatus(&batchJob, jobs))
	}

	return jobStatuses, nil
}

func (s *syncer) buildBatchJobStatus(batchJob *radixv1.RadixBatchJob, allJobs []*batchv1.Job) radixv1.RadixBatchJobStatus {
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
		now := metav1.Now()
		status.Phase = radixv1.BatchJobPhaseStopped
		status.EndTime = &now
		if len(currentStatus) > 0 {
			status.CreationTime = currentStatus[0].CreationTime
			status.StartTime = currentStatus[0].StartTime
		}
		return status
	}

	if jobs := slice.FindAll(allJobs, func(job *batchv1.Job) bool { return isResourceLabeledWithBatchJobName(batchJob.Name, job) }); len(jobs) > 0 {
		job := jobs[0]
		status.CreationTime = &job.CreationTimestamp
		status.Failed = job.Status.Failed

		switch {
		case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobComplete)):
			status.Phase = radixv1.BatchJobPhaseSucceeded
			status.StartTime = job.Status.StartTime
			status.EndTime = job.Status.CompletionTime
		case slice.Any(job.Status.Conditions, isJobStatusCondition(batchv1.JobFailed)):
			status.Phase = radixv1.BatchJobPhaseFailed
			status.StartTime = job.Status.StartTime
			if failedConditions := slice.FindAll(job.Status.Conditions, isJobStatusCondition(batchv1.JobFailed)); len(failedConditions) > 0 {
				status.EndTime = &failedConditions[0].LastTransitionTime
				status.Reason = failedConditions[0].Reason
				status.Message = failedConditions[0].Message
			}
		case job.Status.Active > 0:
			status.Phase = radixv1.BatchJobPhaseActive
			status.StartTime = job.Status.StartTime
		}

	}

	return status
}

func (s *syncer) restoreStatus() error {
	if restoredStatus, ok := s.batch.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.batch.Status).IsZero() {
			var status radixv1.RadixBatchStatus

			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				return fmt.Errorf("unable to restore status for batch %s.%s from annotation: %w", s.batch.GetNamespace(), s.batch.GetName(), err)
			}

			return s.updateStatus(func(currStatus *radixv1.RadixBatchStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}
