package models

import "time"

// ImageHubSecret holds general information about image hubs
// swagger:model ImageHubSecret
type ImageHubSecret struct {
	// Server name of the image hub
	//
	// required: true
	// example: myprivaterepo.azurecr.io
	Server string `json:"server"`

	// Username for connecting to private image hub
	//
	// required: true
	// example: my-user-name
	Username string `json:"username"`

	// Email provided in radixconfig.yaml
	//
	// required: false
	// example: radix@equinor.com
	Email string `json:"email"`

	// Status of the secret
	// - Pending = Secret value is not set
	// - Consistent = Secret value is set
	//
	// required: false
	// enum: Pending,Consistent
	// example: Consistent
	Status string `json:"status"`

	// Updated when the secret was last changed
	//
	// required: false
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated,omitempty"`
}
