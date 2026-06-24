package models

// ScheduledBatchRequest holds information about a creating scheduled batch request
// swagger:model ScheduledBatchRequest
type ScheduledBatchRequest struct {
	// Name of the Radix deployment for a batch
	DeploymentName string `json:"deploymentName"`
}

// ScheduledJobRequest holds information about a creating scheduled job request
// swagger:model ScheduledJobRequest
type ScheduledJobRequest struct {
	// Name of the Radix deployment for a job
	DeploymentName string `json:"deploymentName"`
}
