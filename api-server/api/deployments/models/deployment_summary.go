package models

import "time"

// swagger:ignore
type DeploymentSummaryPipelineJobInfo struct {
	// Name of job creating deployment
	//
	// required: false
	CreatedByJob string `json:"createdByJob,omitempty"`

	// Type of pipeline job
	//
	// required: false
	// enum: build,build-deploy,promote,deploy,apply-config
	// example: build-deploy
	PipelineJobType string `json:"pipelineJobType,omitempty"`

	// Name of the branch used to build the deployment
	//
	// required: false
	// example: main
	BuiltFromBranch string `json:"builtFromBranch,omitempty"`

	// Name of the environment the deployment was promoted from
	// Applies only for pipeline jobs of type 'promote'
	//
	// required: false
	// example: qa
	PromotedFromEnvironment string `json:"promotedFromEnvironment,omitempty"`

	// CommitID the commit ID of the branch to build
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`

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

// DeploymentSummary describe an deployment
// swagger:model DeploymentSummary
type DeploymentSummary struct {
	DeploymentSummaryPipelineJobInfo `json:",inline"`

	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Array of component summaries
	//
	// required: false
	Components []*ComponentSummary `json:"components,omitempty"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: true
	// example: prod
	Environment string `json:"environment"`

	// Status of deployment reconciliation
	//
	// required: true
	Status DeploymentStatus `json:"status"`

	// StatusReason contains details when deployment status is Failed
	//
	// required: false
	StatusReason string `json:"statusReason,omitempty"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: true
	// swagger:strfmt date-time
	ActiveFrom time.Time `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// swagger:strfmt date-time
	ActiveTo *time.Time `json:"activeTo"`

	// GitCommitHash the hash of the git commit from which radixconfig.yaml was parsed
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	GitCommitHash string `json:"gitCommitHash,omitempty"`

	// GitTags the git tags that the git commit hash points to
	//
	// required: false
	// example: "v1.22.1 v1.22.3"
	GitTags string `json:"gitTags,omitempty"`

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

	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	//
	// required: false
	// Extensions:
	// x-nullable: true
	RefreshBuildCache *bool `json:"refreshBuildCache,omitempty"`
}

// DeploymentItem describe a deployment short info
// swagger:model DeploymentItem
type DeploymentItem struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: true
	// swagger:strfmt date-time
	ActiveFrom time.Time `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// swagger:strfmt date-time
	ActiveTo *time.Time `json:"activeTo"`

	// GitCommitHash the hash of the git commit from which radixconfig.yaml was parsed
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	GitCommitHash string `json:"gitCommitHash,omitempty"`
}
