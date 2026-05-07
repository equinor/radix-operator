package models

import (
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// PipelineParametersPromote identify deployment to promote and a target environment
// swagger:model PipelineParametersPromote
type PipelineParametersPromote struct {
	// ID of the deployment to promote
	// REQUIRED for "promote" pipeline
	//
	// example: dev-9tyu1-tftmnqzq
	DeploymentName string `json:"deploymentName"`

	// Name of environment where to look for the deployment to be promoted
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	FromEnvironment string `json:"fromEnvironment"`

	// Name of environment to receive the promoted deployment
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// MapPipelineParametersPromoteToJobParameter maps to JobParameter
func (promoteParam PipelineParametersPromote) MapPipelineParametersPromoteToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		DeploymentName:  promoteParam.DeploymentName,
		FromEnvironment: promoteParam.FromEnvironment,
		ToEnvironment:   promoteParam.ToEnvironment,
		TriggeredBy:     promoteParam.TriggeredBy,
	}
}

// PipelineParametersBuild describe branch to build and its commit ID
// swagger:model PipelineParametersBuild
type PipelineParametersBuild struct {
	// Deprecated: use GitRef instead
	// Branch the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: master
	Branch string `json:"branch"`

	// Name of environment to build for
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment,omitempty"`

	// CommitID the commit ID of the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// PushImage should image be pushed to container registry. Defaults pushing
	//
	// example: true
	PushImage string `json:"pushImage"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`

	// ImageTag of the image - if empty will use default logic
	//
	// example: master-latest
	ImageTag string `json:"imageTag,omitempty"`

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

	// GitRef Branch or tag to build from
	// REQUIRED for "build" and "build-deploy" pipelines
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

// MapPipelineParametersBuildToJobParameter maps to JobParameter
func (buildParam PipelineParametersBuild) MapPipelineParametersBuildToJobParameter() *jobModels.JobParameters {
	gitRef, gitRefType := buildParam.Branch, string(radixv1.GitRefBranch)
	if len(buildParam.GitRef) > 0 {
		gitRef = buildParam.GitRef
	}
	if len(buildParam.GitRefType) > 0 {
		gitRefType = buildParam.GitRefType
	}
	return &jobModels.JobParameters{
		CommitID:              buildParam.CommitID,
		PushImage:             buildParam.PushImageToContainerRegistry(),
		TriggeredBy:           buildParam.TriggeredBy,
		ToEnvironment:         buildParam.ToEnvironment,
		ImageTag:              buildParam.ImageTag,
		OverrideUseBuildCache: buildParam.OverrideUseBuildCache,
		RefreshBuildCache:     buildParam.RefreshBuildCache,
		GitRef:                gitRef,
		GitRefType:            gitRefType,
	}
}

// PushImageToContainerRegistry Normalises the "PushImage" param from a string
func (buildParam PipelineParametersBuild) PushImageToContainerRegistry() bool {
	return buildParam.PushImage != "0" && buildParam.PushImage != "false"
}

// PipelineParametersDeploy describes environment to deploy
// swagger:model PipelineParametersDeploy
type PipelineParametersDeploy struct {
	// Name of environment to deploy
	// REQUIRED for "deploy" pipeline
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment"`

	// Image tags names for components
	//
	// example: {"component1":"tag1", "component2":"tag2"}
	ImageTagNames map[string]string `json:"imageTagNames"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`

	// CommitID the commit ID of the branch
	// OPTIONAL for information only
	//
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`

	// ComponentsToDeploy List of components to deploy
	// OPTIONAL If specified, only these components are deployed
	//
	// required: false
	ComponentsToDeploy []string `json:"componentsToDeploy"`
}

// MapPipelineParametersDeployToJobParameter maps to JobParameter
func (deployParam PipelineParametersDeploy) MapPipelineParametersDeployToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		ToEnvironment:      deployParam.ToEnvironment,
		TriggeredBy:        deployParam.TriggeredBy,
		ImageTagNames:      deployParam.ImageTagNames,
		CommitID:           deployParam.CommitID,
		ComponentsToDeploy: deployParam.ComponentsToDeploy,
	}
}

// PipelineParametersApplyConfig describes base info
// swagger:model PipelineParametersApplyConfig
type PipelineParametersApplyConfig struct {
	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`
}

// MapPipelineParametersApplyConfigToJobParameter maps to JobParameter
func (param PipelineParametersApplyConfig) MapPipelineParametersApplyConfigToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		TriggeredBy:       param.TriggeredBy,
		DeployExternalDNS: param.DeployExternalDNS,
	}
}
