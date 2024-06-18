package batch

import (
	"fmt"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func getKubeServiceName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

func getKubeJobName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

func isResourceLabeledWithBatchJobName(batchJobName string, resource metav1.Object) bool {
	return kubelabels.SelectorFromSet(radixlabels.ForBatchJobName(batchJobName)).Matches(kubelabels.Set(resource.GetLabels()))
}

func isBatchJobStopRequested(batchJob *radixv1.RadixBatchJob) bool {
	return batchJob.Stop != nil && *batchJob.Stop
}

func isJobStatusWaiting(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseWaiting
}

func isBatchDone(batch *radixv1.RadixBatch) bool {
	jobStatusesMap := slice.Reduce(batch.Status.JobStatuses, make(map[string]radixv1.RadixBatchJobStatus), func(acc map[string]radixv1.RadixBatchJobStatus, jobStatus radixv1.RadixBatchJobStatus) map[string]radixv1.RadixBatchJobStatus {
		acc[jobStatus.Name] = jobStatus
		return acc
	})
	for _, batchJob := range batch.Spec.Jobs {
		jobStatus, ok := jobStatusesMap[batchJob.Name]
		if !ok {
			return false
		}
		if !isJobStatusDone(jobStatus) || needRestartJob(batchJob.Restart, jobStatus.Restart) {
			return false
		}
	}
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted
}

func isJobStatusDone(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded ||
		jobStatus.Phase == radixv1.BatchJobPhaseFailed ||
		jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func isBatchJobDone(batch *radixv1.RadixBatch, batchJobName string) bool {
	return slice.Any(batch.Status.JobStatuses,
		func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Name == batchJobName && isJobStatusDone(jobStatus)
		})
}

func ownerReference(job *radixv1.RadixBatch) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: radixv1.SchemeGroupVersion.Identifier(),
			Kind:       radixv1.KindRadixBatch,
			Name:       job.Name,
			UID:        job.UID,
			Controller: utils.BoolPtr(true),
		},
	}
}
