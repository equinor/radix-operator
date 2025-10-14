package internal

import (
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetPodStatusByRadixBatchJobPodStatus converts a slice of RadixBatchJobPodStatus to a slice of PodStatus.
func GetPodStatusByRadixBatchJobPodStatus(radixBatch *radixv1.RadixBatch, podStatuses []radixv1.RadixBatchJobPodStatus) []modelsv1.PodStatus {
	return slice.Map(podStatuses, func(status radixv1.RadixBatchJobPodStatus) modelsv1.PodStatus {
		var created, started, ended *time.Time
		if status.CreationTime != nil {
			created = pointers.Ptr(radixBatch.GetCreationTimestamp().Time)
		}
		if status.StartTime != nil {
			started = &status.StartTime.Time
		}

		if status.EndTime != nil {
			ended = &status.EndTime.Time
		}

		return modelsv1.PodStatus{
			Name:             status.Name,
			Created:          created,
			StartTime:        started,
			EndTime:          ended,
			ContainerStarted: started,
			Status:           modelsv1.ReplicaStatus{Status: string(status.Phase)},
			StatusMessage:    status.Message,
			RestartCount:     status.RestartCount,
			Image:            status.Image,
			ImageId:          status.ImageID,
			PodIndex:         status.PodIndex,
			ExitCode:         status.ExitCode,
			Reason:           status.Reason,
		}
	})
}
