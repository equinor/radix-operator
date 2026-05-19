package utils

import (
	"github.com/equinor/radix-operator/api-server/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetBatchJobStatusByJobApiStatus Get batch job status by job api status
func GetBatchJobStatusByJobApiStatus(status v1.RadixBatchJobApiStatus) models.ScheduledBatchJobStatus {
	switch status {
	case v1.RadixBatchJobApiStatusRunning:
		return models.ScheduledBatchJobStatusRunning
	case v1.RadixBatchJobApiStatusSucceeded:
		return models.ScheduledBatchJobStatusSucceeded
	case v1.RadixBatchJobApiStatusFailed:
		return models.ScheduledBatchJobStatusFailed
	case v1.RadixBatchJobApiStatusWaiting:
		return models.ScheduledBatchJobStatusWaiting
	case v1.RadixBatchJobApiStatusStopping:
		return models.ScheduledBatchJobStatusStopping
	case v1.RadixBatchJobApiStatusStopped:
		return models.ScheduledBatchJobStatusStopped
	case v1.RadixBatchJobApiStatusActive:
		return models.ScheduledBatchJobStatusActive
	case v1.RadixBatchJobApiStatusCompleted:
		return models.ScheduledBatchJobStatusCompleted
	default:
		return ""
	}
}

// GetBatchJobStatusByJobApiCondition Get batch job status by job api condition
func GetBatchJobStatusByJobApiCondition(conditionType v1.RadixBatchConditionType) models.ScheduledBatchJobStatus {
	switch conditionType {
	case v1.BatchConditionTypeWaiting:
		return models.ScheduledBatchJobStatusWaiting
	case v1.BatchConditionTypeActive:
		return models.ScheduledBatchJobStatusActive
	case v1.BatchConditionTypeCompleted:
		return models.ScheduledBatchJobStatusCompleted
	default:
		return ""
	}
}
