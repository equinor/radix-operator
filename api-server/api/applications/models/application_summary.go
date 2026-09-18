package models

import (
	environmentmodels "github.com/equinor/radix-operator/api-server/api/environments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
)

// ApplicationSummary describe an application
// swagger:model ApplicationSummary
type ApplicationSummary struct {
	// Name the name of the application
	//
	// required: true
	// example: radix-canary-golang
	Name string `json:"name"`

	// AppID is the unique identifier for the Radix application. Not to be confused by Configuration Item.
	AppID string `json:"appID"`

	// CloneURL is the URL of the GitHub repository where the Radix configuration file is located.
	// required: true
	CloneURL string `json:"cloneURL"`

	AdGroups []string `json:"adGroups,omitempty"`

	AdUsers []string `json:"adUsers,omitempty"`

	ReaderAdGroups []string `json:"readerAdGroups,omitempty"`

	ReaderAdUsers []string `json:"readerAdUsers,omitempty"`

	Creator string `json:"creator,omitempty"`

	// +optional
	Owner string `json:"owner,omitempty"`

	// ConfigBranch is the branch in the git repository where the Radix configuration file is located.
	// See https://git-scm.com/docs/git-check-ref-format#_description for more details.
	// required: true
	ConfigBranch string `json:"configBranch"`

	// RadixConfigFullName is the full name of the Radix configuration file in the git repository.
	// required: true
	RadixConfigFullName string `json:"radixConfigFullName"`

	// ConfigurationItem is an identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	ConfigurationItem string `json:"configurationItem,omitempty"`

	// LatestJob The latest started job
	//
	// required: false
	LatestJob *jobModels.JobSummary `json:"latestJob,omitempty"`

	// Environments List of environments for this application
	//
	// required: false
	Environments []environmentmodels.Environment `json:"environments,omitempty"`
}
