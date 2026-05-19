package models

// ApplicationRegistrationUpsertResponse describe an application upsert operation response
// swagger:model ApplicationRegistrationUpsertResponse
type ApplicationRegistrationUpsertResponse struct {
	// ApplicationRegistration
	//
	// required: false
	ApplicationRegistration *ApplicationRegistration `json:"applicationRegistration"`

	// Warnings of upsert operation
	//
	// required: false
	// example: ["Repository is used in other application(s)"]
	Warnings []string `json:"warnings,omitempty"`
}
