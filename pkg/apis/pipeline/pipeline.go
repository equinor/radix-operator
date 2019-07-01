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

// Definition Holds pipeline definition
type Definition struct {
	Name  string
	Steps []StepType
}

// GetSupportedPipelines Lists supported pipelines
func GetSupportedPipelines() []Definition {
	return []Definition{
		Definition{BuildDeploy, []StepType{ApplyConfigStep, BuildStep, DeployStep}},
		Definition{Build, []StepType{ApplyConfigStep, BuildStep}},
		Definition{Promote, []StepType{PromoteStep}}}
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (*Definition, error) {
	// Default to build-deploy for backward compatibility
	if name == "" {
		return &Definition{BuildDeploy, []StepType{ApplyConfigStep, BuildStep, DeployStep}}, nil
	}

	for _, pipeline := range GetSupportedPipelines() {
		if pipeline.Name == name {
			return &pipeline, nil
		}
	}

	return nil, fmt.Errorf("No pipeline found by name %s", name)
}
