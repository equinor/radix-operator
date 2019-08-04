package pipeline

import (
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Definition Holds pipeline definition
type Definition struct {
	Type  v1.RadixPipelineType
	Steps []StepType
}

// GetSupportedPipelines Lists supported pipelines
func GetSupportedPipelines() []Definition {
	return []Definition{
		Definition{v1.BuildDeploy, []StepType{ApplyConfigStep, BuildStep, DeployStep}},
		Definition{v1.Build, []StepType{ApplyConfigStep, BuildStep}},
		Definition{v1.Promote, []StepType{PromoteStep}}}
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (*Definition, error) {
	// Default to build-deploy for backward compatibility
	if name == "" {
		return &Definition{v1.BuildDeploy, []StepType{ApplyConfigStep, BuildStep, DeployStep}}, nil
	}

	for _, pipeline := range GetSupportedPipelines() {
		if string(pipeline.Type) == name {
			return &pipeline, nil
		}
	}

	return nil, fmt.Errorf("No pipeline found by name %s", name)
}
