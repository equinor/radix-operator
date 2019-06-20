package pipeline

import "fmt"

// Type Enumeration of the different pipelines we support
type Type int

const (
	// BuildDeploy Will do build based on docker file and deploy to mapped environment
	BuildDeploy Type = iota

	// Build Will do build based on docker file
	Build

	// Promote Will promote a deployment from one environment to another,
	// or an older deployment to an active
	Promote

	// end marker of the enum
	numPipelines
)

func (p Type) String() string {
	return GetPipelines()[p]
}

func GetPipelines() []string {
	return []string{"build-deploy", "build"}
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (Type, error) {
	for pipeline := BuildDeploy; pipeline < numPipelines; pipeline++ {
		if pipeline.String() == name {
			return pipeline, nil
		}
	}

	return numPipelines, fmt.Errorf("No pipeline found by name %s", name)
}
