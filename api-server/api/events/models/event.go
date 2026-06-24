package models

import "time"

// PodState holds information about the state of the first container in a Pod
// swagger:model PodState
type PodState struct {
	// Specifies whether the first container has passed its readiness probe.
	//
	// example: false
	Ready bool `json:"ready"`

	// Specifies whether the first container has started.
	//
	// example: true
	// Extensions:
	// x-nullable: true
	Started *bool `json:"started,omitempty"`

	// The number of times the first container has been restarted
	//
	// example: 1
	RestartCount int32 `json:"restartCount"`
}

// ObjectState holds information about the state of objects involved in an event
// swagger:model ObjectState
type ObjectState struct {
	// Details about the pod state for a pod related event
	Pod *PodState `json:"pod"`
}

// Event holds information about Kubernetes events
// swagger:model Event
type Event struct {

	// The time (ISO8601) at which the event was last recorded
	//
	// swagger:strfmt date-time
	// example: 2020-11-05T13:25:07.000Z
	LastTimestamp time.Time `json:"lastTimestamp"`

	// Kind of object involved in this event
	//
	// example: Pod
	InvolvedObjectKind string `json:"involvedObjectKind"`

	// Namespace of object involved in this event
	//
	// example: myapp-production
	InvolvedObjectNamespace string `json:"involvedObjectNamespace"`

	// Name of object involved in this event
	//
	// example: www-74cb7c986-fgcrl
	InvolvedObjectName string `json:"involvedObjectName"`

	// The state of the involved object
	// Currently only events with type Warning and involvedObjectKind Pod has state information
	// The value is not set if the pod does not exist
	InvolvedObjectState *ObjectState `json:"involvedObjectState,omitempty"`

	// Type of event (Normal, Warning)
	//
	// example: Warning
	Type string `json:"type"`

	// A short, machine understandable string that gives the reason for this event
	//
	// example: Unhealthy
	Reason string `json:"reason"`

	// A human-readable description of the status of this event
	//
	// example: 'Readiness probe failed: dial tcp 10.40.1.5:3003: connect: connection refused'
	Message string `json:"message"`
}
