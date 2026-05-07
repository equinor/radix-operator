package models

// ApplicationRegistrationPatch contains fields that can be patched on a registration
// swagger:model ApplicationRegistrationPatch
type ApplicationRegistrationPatch struct {
	// AdGroups the groups that should be able to access the application
	//
	// required: false
	AdGroups *[]string `json:"adGroups,omitempty"`

	// AdUsers the users/service-principals that should be able to access the application
	//
	// required: false
	AdUsers *[]string `json:"adUsers,omitempty"`

	// ReaderAdGroups the groups that should be able to read the application
	//
	// required: false
	ReaderAdGroups *[]string `json:"readerAdGroups,omitempty"`

	// ReaderAdUsers the users/service-principals that should be able to read the application
	//
	// required: false
	ReaderAdUsers *[]string `json:"readerAdUsers,omitempty"`

	// Owner of the application - should be an email
	//
	// required: false
	Owner *string `json:"owner,omitempty"`

	// Repository the github repository
	//
	// required: false
	Repository *string `json:"repository,omitempty"`

	// WBS information
	//
	// required: false
	WBS *string `json:"wbs,omitempty"`

	// ConfigBranch information
	//
	// required: false
	ConfigBranch *string `json:"configBranch,omitempty"`

	// radixconfig.yaml file name and path, starting from the GitHub repository root (without leading slash)
	//
	// required: false
	RadixConfigFullName string `json:"radixConfigFullName,omitempty"`

	// ConfigurationItem is an identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	//
	// required: false
	ConfigurationItem *string `json:"configurationItem"`
}
