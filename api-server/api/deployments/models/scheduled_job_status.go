package models

// ScheduledBatchJobStatus Enumeration of the statuses of a scheduled job
// swagger:enum ScheduledBatchJobStatus
type ScheduledBatchJobStatus string

const (
	// ScheduledBatchJobStatusRunning Active
	ScheduledBatchJobStatusRunning ScheduledBatchJobStatus = "Running"

	// ScheduledBatchJobStatusSucceeded Job succeeded
	ScheduledBatchJobStatusSucceeded ScheduledBatchJobStatus = "Succeeded"

	// ScheduledBatchJobStatusFailed Job failed
	ScheduledBatchJobStatusFailed ScheduledBatchJobStatus = "Failed"

	// ScheduledBatchJobStatusWaiting Job pending
	ScheduledBatchJobStatusWaiting ScheduledBatchJobStatus = "Waiting"

	// ScheduledBatchJobStatusStopping job is stopping
	ScheduledBatchJobStatusStopping ScheduledBatchJobStatus = "Stopping"

	// ScheduledBatchJobStatusStopped job stopped
	ScheduledBatchJobStatusStopped ScheduledBatchJobStatus = "Stopped"

	// ScheduledBatchJobStatusActive job, one or more pods are not ready
	ScheduledBatchJobStatusActive ScheduledBatchJobStatus = "Active"

	// ScheduledBatchJobStatusCompleted batch jobs are completed
	ScheduledBatchJobStatusCompleted ScheduledBatchJobStatus = "Completed"
)
