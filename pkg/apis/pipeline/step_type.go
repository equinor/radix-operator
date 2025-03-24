package pipeline

import (
	eventsv1 "k8s.io/api/events/v1"
	"regexp"
)

// StepType Enumeration of the different steps a pipeline could contain
type StepType string

const (
	// PreparePipelinesStep Step to prepare pipelines
	PreparePipelinesStep StepType = "prepare-pipelines"

	// ApplyConfigStep Step type to apply radix config
	ApplyConfigStep = "apply-config"

	// BuildStep Step to build the docker image
	BuildStep = "build"

	// DeployStep Step to deploy the RD
	DeployStep = "deploy"

	// PromoteStep Will promote a deployment from one environment to another,
	// or an older deployment to an active
	PromoteStep = "promote"

	// RunPipelinesStep Step to run pipelines
	RunPipelinesStep = "run-pipelines"

	// DeployConfigStep Step to deploy the RD for applied config
	DeployConfigStep = "deploy-config"

	// CreateRadixDeployment Step to create the active RD
	CreateRadixDeployment = "create-deployment"

	// ApplyRadixDeployment Step to apply the active RD to Kubernetes objects
	ApplyRadixDeployment = "apply-deployment"

	jobStepPathPattern = `^status\.steps\{(?:name=)?([^{}]+[^=])\}$`
)

// GetStepType Get step type from a string
func GetStepType(stepType string) (StepType, bool) {
	switch StepType(stepType) {
	case PreparePipelinesStep, ApplyConfigStep, BuildStep, DeployStep, PromoteStep, RunPipelinesStep, DeployConfigStep, CreateRadixDeployment, ApplyRadixDeployment:
		return StepType(stepType), true
	default:
		return "", false
	}
}

// GetStepNameFromRadixJobEvent Get step name from Radix job event
func GetStepNameFromRadixJobEvent(event *eventsv1.Event) (string, bool) {
	if event == nil || len(event.Regarding.FieldPath) == 0 {
		return "", false
	}
	re := regexp.MustCompile(jobStepPathPattern)
	if match := re.FindStringSubmatch(event.Regarding.FieldPath); match != nil {
		return match[1], true
	}
	return "", false
}
