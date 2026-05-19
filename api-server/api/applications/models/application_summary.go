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

	// LatestJob The latest started job
	//
	// required: false
	LatestJob *jobModels.JobSummary `json:"latestJob,omitempty"`

	// Environments List of environments for this application
	//
	// required: false
	Environments []environmentmodels.Environment `json:"environments,omitempty"`
}
