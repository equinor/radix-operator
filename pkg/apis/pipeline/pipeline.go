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
		Definition{v1.BuildDeploy, []StepType{
			PrepareTektonPipelineStep,
			ApplyConfigStep,
			BuildStep,
			RunTektonPipelineStep,
			DeployStep,
			ScanImageStep,
		}},
		Definition{v1.Build, []StepType{
			PrepareTektonPipelineStep,
			ApplyConfigStep,
			BuildStep,
			RunTektonPipelineStep,
			ScanImageStep,
		}},
		Definition{v1.Promote, []StepType{
			PrepareTektonPipelineStep,
			RunTektonPipelineStep,
			PromoteStep}},
		Definition{v1.Deploy, []StepType{
			PrepareTektonPipelineStep,
			ApplyConfigStep,
			RunTektonPipelineStep,
			DeployStep}}}
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

	return nil, fmt.Errorf("No pipeline found by name %s", name)
}
