package pipeline

import "fmt"

// StepType Enumeration of the different steps a pipeline could contain
type StepType int

const (
	// ApplyConfigStep Step type to apply radix config
	ApplyConfigStep StepType = iota

	// BuildStep Step to build the docker image
	BuildStep

	// DeployStep Step to deploy the RD
	DeployStep

	// PromoteStep Will promote a deployment from one environment to another,
	// or an older deployment to an active
	PromoteStep

	// end marker of the enum
	numSteps
)

func (p StepType) String() string {
	return GetSteps()[p]
}

// GetSteps Enumerated list of steps
func GetSteps() []string {
	return []string{"apply-config", "build", "deploy", "promote", "NA"}
}

// GetStepFromName Gets Step from string
func GetStepFromName(name string) (StepType, error) {
	for step := ApplyConfigStep; step < numSteps; step++ {
		if step.String() == name {
			return step, nil
		}
	}

	return numSteps, fmt.Errorf("No step found by name %s", name)
}
