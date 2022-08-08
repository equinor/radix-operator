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
		{v1.BuildDeploy, []StepType{
			PreparePipelinesStep,
			ApplyConfigStep,
			BuildStep,
			RunPipelinesStep,
			DeployStep,
			ScanImageStep,
		}},
		{v1.Build, []StepType{
			PreparePipelinesStep,
			ApplyConfigStep,
			BuildStep,
			RunPipelinesStep,
			ScanImageStep,
		}},
		{v1.Promote, []StepType{
			PreparePipelinesStep,
			RunPipelinesStep,
			PromoteStep}},
		{v1.Deploy, []StepType{
			PreparePipelinesStep,
			ApplyConfigStep,
			RunPipelinesStep,
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

	return nil, fmt.Errorf("no pipeline found by name %s", name)
}
