package models

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Running Active
	Running ProgressStatus = iota

	// Succeeded Job/step succeeded
	Succeeded

	// Failed Job/step failed
	Failed

	// Waiting Job/step pending
	Waiting

	// Stopping job
	Stopping

	// Stopped job
	Stopped

	// StoppedNoChanges The job is stopped due to no changes in build components
	StoppedNoChanges

	numStatuses
)

func (p ProgressStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Running", "Succeeded", "Failed", "Waiting", "Stopping", "Stopped", "StoppedNoChanges"}[p]
}

// GetStatusFromRadixJobStatus Returns job status as string
func GetStatusFromRadixJobStatus(jobStatus v1.RadixJobStatus, specStop bool) string {
	if specStop {
		if string(jobStatus.Condition) == Stopped.String() || string(jobStatus.Condition) == Failed.String() || string(jobStatus.Condition) == Succeeded.String() {
			return Stopped.String()
		}
		return Stopping.String()
	}

	if jobStatus.Condition != "" {
		return string(jobStatus.Condition)
	}

	// radix-operator still hasn't picked up the job
	return Waiting.String()
}
