package models

import (
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
)

const (
	RadixPipelineJobRerunAnnotation = "radix.equinor.com/rerun-pipeline-job-from"
)

// Job holds general information about job
// swagger:model Job
type Job struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Branch to build from
	//
	// required: false
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// Image tags names for components - if empty will use default logic
	//
	// required: false
	// example: {"component1":"tag1", "component2":"tag2"}
	ImageTagNames map[string]string `json:"imageTagNames,omitempty"`

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

	// TriggeredBy user that triggered the job. If through webhook = sender.login. If through api = usertoken.upn
	//
	// required: false
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy"`

	// RerunFromJob The source name of the job if this job was restarted from it
	//
	// required: false
	// example: radix-pipeline-20231011104617-urynf
	RerunFromJob string `json:"rerunFromJob"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Status of the job
	//
	// required: false
	// enum: Queued,Waiting,Running,Succeeded,Failed,Stopped,Stopping,StoppedNoChanges
	// example: Waiting
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// enum: build,build-deploy,promote,deploy,apply-config
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// RadixDeployment name, which is promoted
	//
	// required: false
	PromotedFromDeployment string `json:"promotedFromDeployment,omitempty"`

	// PromotedFromEnvironment the name of the environment that was promoted from
	//
	// required: false
	// example: dev
	PromotedFromEnvironment string `json:"promotedFromEnvironment,omitempty"`

	// PromotedToEnvironment the name of the environment that was promoted to
	//
	// required: false
	// example: qa
	PromotedToEnvironment string `json:"promotedToEnvironment,omitempty"`

	// DeployedToEnvironment the name of the environment that was deployed to
	//
	// required: false
	// example: qa
	DeployedToEnvironment string `json:"deployedToEnvironment,omitempty"`

	// Array of steps
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Step"
	Steps []Step `json:"steps"`

	// Array of deployments
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/DeploymentSummary"
	Deployments []*deploymentModels.DeploymentSummary `json:"deployments,omitempty"`

	// Components (array of ComponentSummary) created by the job
	//
	// Deprecated: Inspect each deployment to get list of components created by the job
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/ComponentSummary"
	Components []*deploymentModels.ComponentSummary `json:"components,omitempty"`

	// Enables BuildKit when building Dockerfile.
	//
	// required: false
	// Extensions:
	// x-nullable: true
	UseBuildKit *bool `json:"useBuildKit,omitempty"`

	// Defaults to true and requires useBuildKit to have an effect.
	//
	// required: false
	// Extensions:
	// x-nullable: true
	UseBuildCache *bool `json:"useBuildCache,omitempty"`

	// OverrideUseBuildCache override default or configured build cache option
	//
	// required: false
	// Extensions:
	// x-nullable: true
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	//
	// required: false
	// Extensions:
	// x-nullable: true
	RefreshBuildCache *bool `json:"refreshBuildCache,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`

	// TriggeredFromWebhook If true, the job was triggered from a webhook
	//
	// required: true
	TriggeredFromWebhook bool `json:"triggeredFromWebhook"`

	// GitRef Branch or tag to build from
	//
	// required: false
	// example: master
	GitRef string `json:"gitRef,omitempty"`

	// GitRefType When the pipeline job should be built from branch or tag specified in GitRef:
	// - branch
	// - tag
	// - <empty> - either branch or tag
	//
	// required: false
	// enum: branch,tag,""
	// example: branch
	GitRefType string `json:"gitRefType,omitempty"`
}
