package models

import "time"

// ScheduledJobSummary holds general information about scheduled job
// swagger:model ScheduledJobSummary
type ScheduledJobSummary struct {
	// Name of the scheduled job
	//
	// required: true
	// example: job-component-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Created *time.Time `json:"created"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Ended *time.Time `json:"ended"`

	// Status of the job
	//
	// required: true
	// example: Waiting
	Status ScheduledBatchJobStatus `json:"status"`

	// Message of a status, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// Array of ReplicaSummary
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList,omitempty"`

	// JobId JobId, if any
	//
	// required: false
	// example: "job1"
	JobId string `json:"jobId,omitempty"`

	// BatchName Batch name, if any
	//
	// required: false
	// example: "batch-abc"
	BatchName string `json:"batchName,omitempty"`

	// TimeLimitSeconds How long the job supposed to run at maximum
	//
	// required: false
	// example: 3600
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// BackoffLimit Amount of retries due to a logical error in configuration etc.
	//
	// required: true
	// example: 1
	BackoffLimit int32 `json:"backoffLimit"`

	// Resources Resource requirements for the job
	//
	// required: false
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Node Defines node attributes, where pod should be scheduled
	//
	// required: false
	Node *Node `json:"node,omitempty"`

	// Runtime requirements for the batch job
	Runtime *Runtime `json:"runtime,omitempty"`

	// DeploymentName name of RadixDeployment for the job
	//
	// required: true
	DeploymentName string `json:"deploymentName"`

	// FailedCount is the number of times the job has failed
	//
	// required: true
	// example: 1
	FailedCount int32 `json:"failedCount"`

	// Timestamp of the job restart, if applied.
	// +optional
	Restart string

	// Variable names map to values specified for this job.
	//
	// required: false
	Variables map[string]string `json:"variables,omitempty"`

	// Command is the entrypoint array specified for the job. Not executed within a shell.
	//
	// required: false
	Command []string `json:"command,omitempty"`

	// Args to the entrypoint specified for the job.
	//
	// required: false
	Args []string `json:"args,omitempty"`
}

// ScheduledBatchSummary holds information about scheduled batch
// swagger:model ScheduledBatchSummary
type ScheduledBatchSummary struct {
	// Name of the scheduled batch
	//
	// required: true
	// example: batch-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Defines a user defined ID of the batch.
	//
	// required: false
	BatchId string `json:"batchId,omitempty"`

	// Created timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Created *time.Time `json:"created"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Ended *time.Time `json:"ended"`

	// Status of the job
	//
	// required: true
	// example: Waiting
	Status ScheduledBatchJobStatus `json:"status"`

	// TotalJobCount count of jobs, requested to be scheduled by a batch
	//
	// required: true
	// example: 5
	TotalJobCount int `json:"totalJobCount"`

	// Jobs within the batch of ScheduledJobSummary
	//
	// required: false
	JobList []ScheduledJobSummary `json:"jobList,omitempty"`

	// DeploymentName name of RadixDeployment for the batch
	//
	// required: true
	DeploymentName string `json:"deploymentName"`
}
