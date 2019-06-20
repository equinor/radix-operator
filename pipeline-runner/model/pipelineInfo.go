package model

import (
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Type               *pipeline.Type
	RadixRegistration  *v1.RadixRegistration
	RadixApplication   *v1.RadixApplication
	TargetEnvironments map[string]bool
	BranchIsMapped     bool
	JobName            string
	Branch             string
	CommitID           string
	ImageTag           string
	UseCache           string
	PushImage          bool
	Steps              []Step
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Type,
	rr *v1.RadixRegistration,
	ra *v1.RadixApplication,
	targetEnv map[string]bool,
	branchIsMapped bool,
	jobName,
	branch,
	commitID,
	imageTag,
	useCache,
	pushImage string,
	configApplier Step,
	builder Step,
	deployer Step) (PipelineInfo, error) {

	pushImagebool := pipelineType.Name == pipeline.BuildDeploy || !(pushImage == "false" || pushImage == "0") // build and deploy require push

	return PipelineInfo{
		Type:               pipelineType,
		RadixRegistration:  rr,
		RadixApplication:   ra,
		TargetEnvironments: targetEnv,
		BranchIsMapped:     branchIsMapped,
		JobName:            jobName,
		Branch:             branch,
		CommitID:           commitID,
		ImageTag:           imageTag,
		UseCache:           useCache,
		PushImage:          pushImagebool,
		Steps:              getStepsFromType(pipelineType, configApplier, builder, deployer),
	}, nil
}

func getStepsFromType(pipelineType *pipeline.Type, allStepImplementations ...Step) []Step {
	stepImplementations := make([]Step, 0)

	for _, step := range pipelineType.Steps {
		for _, stepImplementation := range allStepImplementations {
			stepType := stepImplementation.ImplementationForType()

			if stepType == step {
				stepImplementations = append(stepImplementations, stepImplementation)
				break
			}
		}
	}

	return stepImplementations
}

// GetAppName Gets name of app from registration
func (pipelineInfo PipelineInfo) GetAppName() string {
	return pipelineInfo.RadixRegistration.GetName()
}
