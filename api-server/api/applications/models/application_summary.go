package models

import (
	environmentmodels "github.com/equinor/radix-operator/api-server/api/environments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	AppID radixv1.ULID `json:"appID"`

	// CloneURL is the URL of the GitHub repository where the Radix configuration file is located.
	// +required
	CloneURL string `json:"cloneURL"`

	// +optional
	AdGroups []string `json:"adGroups,omitempty"`

	// +optional
	AdUsers []string `json:"adUsers,omitempty"`

	// +optional
	ReaderAdGroups []string `json:"readerAdGroups,omitempty"`

	// +optional
	ReaderAdUsers []string `json:"readerAdUsers,omitempty"`

	// +optional
	Creator string `json:"creator,omitempty"`

	// +optional
	Owner string `json:"owner,omitempty"`

	// ConfigBranch is the branch in the git repository where the Radix configuration file is located.
	// See https://git-scm.com/docs/git-check-ref-format#_description for more details.
	// +required
	ConfigBranch string `json:"configBranch"`

	// RadixConfigFullName is the full name of the Radix configuration file in the git repository.
	// +optional
	RadixConfigFullName string `json:"radixConfigFullName,omitempty"`

	// ConfigurationItem is and identifier for an entity in a configuration management solution such as a CMDB.
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
