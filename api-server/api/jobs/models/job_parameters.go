package models

// JobParameters parameters to create a pipeline job
// Not exposed in the API
type JobParameters struct {
	// Deprecated: use GitRef instead
	// For build pipeline: Name of the branch
	Branch string `json:"branch"`

	// For build pipeline: Commit ID of the branch
	CommitID string `json:"commitID"`

	// For build pipeline: Should image be pushed to container registry
	PushImage bool `json:"pushImage"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	TriggeredBy string `json:"triggeredBy"`

	// For promote pipeline: Name (ID) of deployment to promote
	DeploymentName string `json:"deploymentName"`

	// For promote pipeline: Environment to locate deployment to promote
	FromEnvironment string `json:"fromEnvironment"`

	// For build or promote pipeline: Target environment for building and promotion
	ToEnvironment string `json:"toEnvironment"`

	// ImageTag of the image - if empty will use default logic
	ImageTag string

	// ImageTagNames tags for components - if empty will use default logic
	ImageTagNames map[string]string

	// ComponentsToDeploy List of components to deploy
	// OPTIONAL If specified, only these components are deployed
	ComponentsToDeploy []string `json:"componentsToDeploy"`

	// OverrideUseBuildCache override default or configured build cache option
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	RefreshBuildCache *bool `json:"refreshBuildCache,omitempty"`

	// DeployExternalDNS deploy external DNS
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`

	// GitRef Branch or tag to build from
	GitRef string `json:"gitRef,omitempty"`

	// GitRefType When the pipeline job should be built from branch or tag specified in GitRef:
	GitRefType string `json:"gitRefType,omitempty"`
}

// GetPushImageTag Represents boolean as 1 or 0
func (param JobParameters) GetPushImageTag() string {
	if param.PushImage {
		return "1"
	}

	return "0"
}
