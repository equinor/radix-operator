package pipeline

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/steps"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Definition Holds pipeline definition
type Definition struct {
	Type  v1.RadixPipelineType
	Steps []steps.StepType
}

// GetSupportedPipelines Lists supported pipelines
func GetSupportedPipelines() []Definition {
	return []Definition{
		{v1.BuildDeploy, []steps.StepType{
			steps.PreparePipelinesStep,
			steps.ApplyConfigStep,
			steps.BuildStep,
			steps.RunPipelinesStep,
			steps.DeployStep,
		}},
		{v1.Build, []steps.StepType{
			steps.PreparePipelinesStep,
			steps.ApplyConfigStep,
			steps.BuildStep,
			steps.RunPipelinesStep,
		}},
		{v1.Promote, []steps.StepType{
			steps.PreparePipelinesStep,
			steps.ApplyConfigStep,
			steps.RunPipelinesStep,
			steps.PromoteStep}},
		{v1.Deploy, []steps.StepType{
			steps.PreparePipelinesStep,
			steps.ApplyConfigStep,
			steps.RunPipelinesStep,
			steps.DeployStep}},
		{v1.ApplyConfig, []steps.StepType{
			steps.PreparePipelinesStep,
			steps.ApplyConfigStep}},
	}
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (*Definition, error) {
	// Default to build-deploy for backward compatibility
	if name == "" {
		name = string(v1.BuildDeploy)
	}

	for _, pipeline := range GetSupportedPipelines() {
		if string(pipeline.Type) == name {
			return &pipeline, nil
		}
	}

	return nil, fmt.Errorf("no pipeline found by name %s", name)
}
