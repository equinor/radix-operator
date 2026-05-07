package models

// ApplicationRegistrationRequest describe a register application request
// swagger:model ApplicationRegistrationRequest
type ApplicationRegistrationRequest struct {
	// ApplicationRegistration
	//
	// required: false
	ApplicationRegistration *ApplicationRegistration `json:"applicationRegistration"`

	// AcknowledgeWarnings acknowledge all warnings
	//
	// required: false
	AcknowledgeWarnings bool `json:"acknowledgeWarnings,omitempty"`
}
