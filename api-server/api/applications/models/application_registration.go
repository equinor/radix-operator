package models

// ApplicationRegistration describe an application
// swagger:model ApplicationRegistration
type ApplicationRegistration struct {
	// Name the unique name of the Radix application
	//
	// required: true
	// example: radix-canary-golang
	Name string `json:"name"`

	// AppID the unique application ID, which is a ULID
	//
	// required: true
	// example: 01JZ5GSH4B388RYMRYZPNR0104
	AppID string `json:"appId"`

	// Repository the github repository
	//
	// required: true
	// example: https://github.com/equinor/radix-canary-golang
	Repository string `json:"repository"`

	// SharedSecret the shared secret of the webhook
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`

	// AdGroups the groups that should be able to access the application
	//
	// required: true
	AdGroups []string `json:"adGroups"`

	// AdUsers the users/service-principals that should be able to access the application
	//
	// required: true
	AdUsers []string `json:"adUsers"`

	// ReaderAdGroups the groups that should be able to read the application
	//
	// required: true
	ReaderAdGroups []string `json:"readerAdGroups"`

	// ReaderAdUsers the users/service-principals that should be able to read the application
	//
	// required: true
	ReaderAdUsers []string `json:"readerAdUsers"`

	// Owner of the application (email). Can be a single person or a shared group email
	//
	// required: true
	Owner string `json:"owner"`

	// Owner of the application (email). Can be a single person or a shared group email
	//
	// required: true
	Creator string `json:"creator"`

	// ConfigBranch information
	//
	// required: true
	ConfigBranch string `json:"configBranch"`

	// radixconfig.yaml file name and path, starting from the GitHub repository root (without leading slash)
	//
	// required: false
	RadixConfigFullName string `json:"radixConfigFullName,omitempty"`

	// ConfigurationItem is an identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	//
	// required: false
	ConfigurationItem string `json:"configurationItem"`
}
