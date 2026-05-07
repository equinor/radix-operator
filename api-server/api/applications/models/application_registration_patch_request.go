package models

// ApplicationRegistrationPatchRequest contains request with fields that can be patched on a registration
// swagger:model ApplicationRegistrationPatchRequest
type ApplicationRegistrationPatchRequest struct {
	// ApplicationRegistration
	//
	// required: true
	ApplicationRegistrationPatch *ApplicationRegistrationPatch `json:"applicationRegistrationPatch"`

	// AcknowledgeWarnings acknowledge all warnings
	//
	// required: false
	AcknowledgeWarnings bool `json:"acknowledgeWarnings,omitempty"`
}
