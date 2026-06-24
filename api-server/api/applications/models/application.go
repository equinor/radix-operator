package models

import (
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
)

// Application details of an application
// swagger:model Application
type Application struct {
	// Name the name of the application
	//
	// required: true
	// example: radix-canary-golang
	Name string `json:"name"`

	// Registration registration details
	//
	// required: true
	Registration ApplicationRegistration `json:"registration"`

	// Environments List of environments for this application
	//
	// required: false
	Environments []*environmentModels.EnvironmentSummary `json:"environments"`

	// Jobs list of run jobs for the application
	//
	// required: false
	Jobs []*jobModels.JobSummary `json:"jobs"`

	// DNS aliases showing nicer endpoint for application, without "app." subdomain domain
	//
	// required: false
	DNSAliases []DNSAlias `json:"dnsAliases,omitempty"`

	// List of external DNS names and which component and environment incoming requests shall be routed to.
	//
	// required: false
	DNSExternalAliases []DNSExternalAlias `json:"dnsExternalAliases,omitempty"`

	// App alias showing nicer endpoint for application
	//
	// required: false
	AppAlias *ApplicationAlias `json:"appAlias,omitempty"`

	// UserIsAdmin if user is member of application's admin groups
	//
	// required: true
	UserIsAdmin bool `json:"userIsAdmin"`

	// UseBuildKit if buildkit is used for building the application
	//
	// required: true
	UseBuildKit bool `json:"useBuildKit"`

	// UseBuildCache if build cache is used for building the application. Applicable when UseBuildKit is true. Default is true.
	//
	// required: true
	UseBuildCache bool `json:"useBuildCache"`
}
