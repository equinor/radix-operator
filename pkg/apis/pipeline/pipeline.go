package pipeline

import "fmt"

const (
	// BuildDeploy Identifyer
	BuildDeploy = "build-deploy"

	// Build Identifyer
	Build = "build"

	// Promote Identifyer
	Promote = "promote"
)

// Type Enumeration of the different pipelines we support
type Type struct {
	Name  string
	Steps []StepType
}

var buildDeployPipeline = Type{BuildDeploy, []StepType{ApplyConfigStep, BuildStep, DeployStep}}
var buildPipeline = Type{Build, []StepType{ApplyConfigStep, BuildStep}}
var promotePipeline = Type{Promote, []StepType{PromoteStep}}

// GetSupportedPipelines Lists supported pipelines
func GetSupportedPipelines() []Type {
	return []Type{
		buildDeployPipeline,
		buildPipeline,
		promotePipeline}
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (*Type, error) {
	// Default to build-deploy for backward compatibility
	if name == "" {
		return &buildDeployPipeline, nil
	}

	for _, pipeline := range GetSupportedPipelines() {
		if pipeline.Name == name {
			return &pipeline, nil
		}
	}

	return nil, fmt.Errorf("No pipeline found by name %s", name)
}
