package events

import v1 "github.com/equinor/radix-operator/job-scheduler/models/v1"

// BatchEvent holds general information about batch event on change of status
// swagger:model BatchEvent
type BatchEvent struct {
	// BatchStatus Batch job status
	v1.BatchStatus

	// Event Event type
	//
	// required: true
	// example: "Create"
	Event Event `json:"event,omitempty"`
}

type Event string

const (
	Create Event = "Create"
	Update Event = "Update"
	Delete Event = "Delete"
)
