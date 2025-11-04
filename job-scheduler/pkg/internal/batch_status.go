package internal

import (
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetBatchName Get batch name from radix batch
func GetBatchName(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.GetName())
}

// GetBatchId returns the batch ID for a Radix batch.
func GetBatchId(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.Spec.BatchId)
}

// IsRadixBatchJobSucceeded Check if Radix batch job is succeeded
func IsRadixBatchJobSucceeded(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded || jobStatus.Phase == radixv1.BatchJobPhaseStopped
}
