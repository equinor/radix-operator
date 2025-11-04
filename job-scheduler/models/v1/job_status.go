package v1

import "time"

// JobStatus holds general information about job status
// swagger:model JobStatus
type JobStatus struct {
	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

	// BatchName Optional Batch ID of a job
	//
	// required: false
	// example: 'batch1'
	BatchName string `json:"batchName,omitempty"`

	// Defines a user defined ID of the batch.
	//
	// required: false
	// example: 'batch-id-1'
	BatchId string `json:"batchId,omitempty"`

	// Name of the job
	// required: true
	// example: calculator
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
	// - Running = Job is running
	// - Succeeded = Job has succeeded
	// - Failed = Job has failed
	// - Waiting = Job is waiting
	// - Stopping = Job is stopping
	// - Stopped = Job has been stopped
	// - Active = Job is active
	// - Completed = Job is completed
	//
	// required: false
	// Enum: Running,Succeeded,Failed,Waiting,Stopping,Stopped,Active,Completed
	// example: Waiting
	Status string `json:"status,omitempty"`

	// Message, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// Updated timestamp when the status was updated
	//
	// required: false
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated"`

	// The number of times the container for the job has failed.
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// Timestamp of the job restart, if applied.
	// +optional
	Restart string `json:"restart,omitempty"`

	// PodStatuses for each pod of the job
	// required: false
	PodStatuses []PodStatus `json:"podStatuses,omitempty"`

	// DeploymentName for this batch
	// required: false
	DeploymentName string
}

// PodStatus contains details for the current status of the job's pods.
// swagger:model PodStatus
type PodStatus struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Created *time.Time `json:"created,omitempty"`

	// The time at which the batch job's pod startedAt
	//
	// required: false
	// swagger:strfmt date-time
	StartTime *time.Time `json:"startTime,omitempty"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	EndTime *time.Time `json:"endTime,omitempty"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	ContainerStarted *time.Time `json:"containerStarted,omitempty"`

	// Status describes the component container status
	//
	// required: false
	Status ReplicaStatus `json:"replicaStatus,omitempty"`

	// StatusMessage provides message describing the status of a component container inside a pod
	//
	// required: false
	StatusMessage string `json:"statusMessage,omitempty"`

	// RestartCount count of restarts of a component container inside a pod
	//
	// required: false
	RestartCount int32 `json:"restartCount,omitempty"`

	// The image the container is running.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image,omitempty"`

	// ImageID of the container's image.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server@sha256:d40cda01916ef63da3607c03785efabc56eb2fc2e0dab0726b1a843e9ded093f
	ImageId string `json:"imageId,omitempty"`

	// The index of the pod in the re-starts
	PodIndex int `json:"podIndex,omitempty"`

	// Exit status from the last termination of the container
	ExitCode int32 `json:"exitCode"`

	// A brief CamelCase message indicating details about why the job is in this phase
	Reason string `json:"reason,omitempty"`
}
