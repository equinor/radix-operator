package pipeline

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
)
