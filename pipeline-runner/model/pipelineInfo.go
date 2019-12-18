package model

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Definition         *pipeline.Definition
	TargetEnvironments map[string]bool
	BranchIsMapped     bool
	PipelineArguments  PipelineArguments
	Steps              []Step

	// Holds information on the images referred to by their respective components
	ComponentImages map[string]pipeline.ComponentImage
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

	// Images used for copying radix config/building/scanning
	ConfigToMap  string
	ImageBuilder string
	ImageScanner string

	// Used for tagging metainformation
	Clustertype string
	Clustername string
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

	configToMap := args[defaults.RadixConfigToMapEnvironmentVariable]
	imageBuilder := args[defaults.RadixImageBuilderEnvironmentVariable]
	imageScanner := args[defaults.RadixImageScannerEnvironmentVariable]
	clusterType := args[defaults.RadixClusterTypeEnvironmentVariable]
	clusterName := args[defaults.ClusternameEnvironmentVariable]

	if branch == "" {
		branch = "dev"
	}
	if imageTag == "" {
		imageTag = "latest"
	}
	if useCache == "" {
		useCache = "true"
	}

	pushImagebool := pipelineType == string(v1.BuildDeploy) || !(pushImage == "false" || pushImage == "0") // build and deploy require push

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
		ConfigToMap:     configToMap,
		ImageBuilder:    imageBuilder,
		ImageScanner:    imageScanner,
		Clustertype:     clusterType,
		Clustername:     clusterName,
	}
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
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
