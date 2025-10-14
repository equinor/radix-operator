package common

// +kubebuilder:object:generate=true

type StatusReason string

const (
	StatusSuccess = "Success"
	StatusFailure = "Failure"
	// StatusReasonUnknown means the server has declined to indicate a specific reason.
	// Status code 500.
	StatusReasonUnknown StatusReason = "InternalError"
	// StatusReasonBadRequest means that the operation could not be fulfilled.
	// Status code 400
	StatusReasonBadRequest StatusReason = "BadRequest"
	// StatusReasonNotFound means one or more resources required for this operation
	// could not be found.
	// Status code 404
	StatusReasonNotFound StatusReason = "NotFound"
	// StatusReasonInvalid means the requested create or update operation cannot be
	// completed due to invalid data provided as part of the request. The client may
	// need to alter the request.
	// Status code 422
	StatusReasonInvalid StatusReason = "Invalid"
)

// Status is a return value for calls that don't return other objects or when a request returns an error
// swagger:model Status
type Status struct {

	// Status of the operation.
	// One of: "Success" or "Failure".
	// example: Failure
	Status string `json:"status,omitempty"`
	// A human-readable description of the status of this operation.
	// required: false
	// example: job job123 is not found
	Message string `json:"message,omitempty"`
	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available. A Reason clarifies an HTTP status
	// code but does not override it.
	// required: false
	// example: NotFound
	Reason StatusReason `json:"reason,omitempty"`

	// Suggested HTTP return code for this status, 0 if not set.
	// required: false
	// example: 404
	Code int `json:"code,omitempty"`
}
