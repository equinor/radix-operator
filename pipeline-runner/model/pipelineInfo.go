package model

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Definition         *pipeline.Definition
	RadixRegistration  *v1.RadixRegistration
	RadixApplication   *v1.RadixApplication
	TargetEnvironments map[string]bool
	BranchIsMapped     bool
	PipelineArguments  PipelineArguments
	Steps              []Step
}

// PipelineArguments Holds arguments for the pipeline
type PipelineArguments struct {
	PipelineType    string
	JobName         string
	Branch          string
	CommitID        string
	ImageTag        string
	UseCache        string
	PushImage       bool
	DeploymentName  string
	FromEnvironment string
	ToEnvironment   string
}

// GetPipelineArgsFromArguments Gets pipeline arguments from arg string
func GetPipelineArgsFromArguments(args map[string]string) PipelineArguments {
	branch := args["BRANCH"]
	commitID := args["COMMIT_ID"]
	imageTag := args["IMAGE_TAG"]
	jobName := args["JOB_NAME"]
	useCache := args["USE_CACHE"]
	pipelineType := args["PIPELINE_TYPE"] // string(model.Build)
	pushImage := args["PUSH_IMAGE"]       // "0"

	deploymentName := args["DEPLOYMENT_NAME"]   // For promotion pipeline
	fromEnvironment := args["FROM_ENVIRONMENT"] // For promotion pipeline
	toEnvironment := args["TO_ENVIRONMENT"]     // For promotion pipeline

	if branch == "" {
		branch = "dev"
	}
	if imageTag == "" {
		imageTag = "latest"
	}
	if useCache == "" {
		useCache = "true"
	}

	pushImagebool := pipelineType == pipeline.BuildDeploy || !(pushImage == "false" || pushImage == "0") // build and deploy require push

	return PipelineArguments{
		PipelineType:    pipelineType,
		JobName:         jobName,
		Branch:          branch,
		CommitID:        commitID,
		ImageTag:        imageTag,
		UseCache:        useCache,
		PushImage:       pushImagebool,
		DeploymentName:  deploymentName,
		FromEnvironment: fromEnvironment,
		ToEnvironment:   toEnvironment,
	}
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
	rr *v1.RadixRegistration,
	ra *v1.RadixApplication,
	targetEnv map[string]bool,
	branchIsMapped bool,
	pipelineArguments PipelineArguments,
	stepImplementations ...Step) (*PipelineInfo, error) {

	stepImplementationsForType, err := getStepstepImplementationsFromType(pipelineType, stepImplementations...)
	if err != nil {
		return nil, err
	}

	return &PipelineInfo{
		Definition:         pipelineType,
		RadixRegistration:  rr,
		RadixApplication:   ra,
		TargetEnvironments: targetEnv,
		BranchIsMapped:     branchIsMapped,
		PipelineArguments:  pipelineArguments,
		Steps:              stepImplementationsForType,
	}, nil
}

func getStepstepImplementationsFromType(pipelineType *pipeline.Definition, allStepImplementations ...Step) ([]Step, error) {
	stepImplementations := make([]Step, 0)

	for _, step := range pipelineType.Steps {
		stepImplementation := getStepImplementationForStepType(step, allStepImplementations)
		if stepImplementation == nil {
			return nil, fmt.Errorf("No step implementation found by type %s", stepImplementation)
		}

		stepImplementations = append(stepImplementations, stepImplementation)
	}

	return stepImplementations, nil
}

func getStepImplementationForStepType(stepType pipeline.StepType, allStepImplementations []Step) Step {
	for _, stepImplementation := range allStepImplementations {
		implementsType := stepImplementation.ImplementationForType()

		if stepType == implementsType {
			return stepImplementation
		}
	}

	return nil
}

// GetAppName Gets name of app from registration
func (pipelineInfo PipelineInfo) GetAppName() string {
	return pipelineInfo.RadixRegistration.GetName()
}
