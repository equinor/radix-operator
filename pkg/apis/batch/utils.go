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

func isResourceLabeledWithBatchJobName(batchJobName string, resource metav1.Object) bool {
	return kubelabels.SelectorFromSet(radixlabels.ForBatchJobName(batchJobName)).Matches(kubelabels.Set(resource.GetLabels()))
}

func isBatchJobStopRequested(batchJob *radixv1.RadixBatchJob) bool {
	return batchJob.Stop != nil && *batchJob.Stop
}

func getKubeServiceName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

func getKubeJobName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

func isBatchJobPhaseDone(phase radixv1.RadixBatchJobPhase) bool {
	return phase == radixv1.BatchJobPhaseSucceeded ||
		phase == radixv1.BatchJobPhaseFailed ||
		phase == radixv1.BatchJobPhaseStopped
}

func isBatchDone(batch *radixv1.RadixBatch) bool {
	if batch.Status.Condition.Type != radixv1.BatchConditionTypeCompleted {
		return false
	}
	return !slice.Any(batch.Spec.Jobs, func(batchJob radixv1.RadixBatchJob) bool {
		return len(batchJob.Restart) > 0
	})
}

func isBatchJobDone(batch *radixv1.RadixBatch, batchJobName string) bool {
	return slice.Any(batch.Status.JobStatuses,
		func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Name == batchJobName && isBatchJobPhaseDone(jobStatus.Phase)
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
