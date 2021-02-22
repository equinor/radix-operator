package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
)

const multiComponentImageName = "multi-component"

type componentType struct {
	name           string
	context        string
	dockerFileName string
}

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Definition        *pipeline.Definition
	RadixApplication  *v1.RadixApplication
	PipelineArguments PipelineArguments
	Steps             []Step

	// Container registry to build with
	ContainerRegistry string

	// Temporary data
	RadixConfigMapName string
	TargetEnvironments map[string]bool
	BranchIsMapped     bool

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
	RadixConfigFile string

	// Images used for copying radix config/building/scanning
	ConfigToMap  string
	ImageBuilder string
	ImageScanner string

	// Used for tagging metainformation
	Clustertype string
	Clustername string

	// Used to indicate debugging session
	Debug bool
}

// GetPipelineArgsFromArguments Gets pipeline arguments from arg string
func GetPipelineArgsFromArguments(args map[string]string) PipelineArguments {
	radixConfigFile := args["RADIX_FILE_NAME"]
	branch := args["BRANCH"]
	commitID := args["COMMIT_ID"]
	imageTag := args["IMAGE_TAG"]
	jobName := args["JOB_NAME"]
	useCache := args["USE_CACHE"]
	pipelineType := args["PIPELINE_TYPE"] // string(model.Build)
	pushImage := args["PUSH_IMAGE"]       // "0"

	deploymentName := args["DEPLOYMENT_NAME"]   // For promotion pipeline
	fromEnvironment := args["FROM_ENVIRONMENT"] // For promotion pipeline
	toEnvironment := args["TO_ENVIRONMENT"]     // For promotion and deploy pipeline

	configToMap := args[defaults.RadixConfigToMapEnvironmentVariable]
	imageBuilder := args[defaults.RadixImageBuilderEnvironmentVariable]
	imageScanner := args[defaults.RadixImageScannerEnvironmentVariable]
	clusterType := args[defaults.RadixClusterTypeEnvironmentVariable]
	clusterName := args[defaults.ClusternameEnvironmentVariable]

	// Indicates that we are debugging the application
	debug, _ := strconv.ParseBool(args["DEBUG"])

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
		RadixConfigFile: radixConfigFile,
		Debug:           debug,
	}
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
	pipelineArguments PipelineArguments,
	stepImplementations ...Step) (*PipelineInfo, error) {

	timestamp := time.Now().Format("20060102150405")
	radixConfigMapName := fmt.Sprintf("radix-config-2-map-%s-%s", timestamp, pipelineArguments.ImageTag)

	stepImplementationsForType, err := getStepstepImplementationsFromType(pipelineType, stepImplementations...)
	if err != nil {
		return nil, err
	}

	return &PipelineInfo{
		Definition:         pipelineType,
		PipelineArguments:  pipelineArguments,
		Steps:              stepImplementationsForType,
		RadixConfigMapName: radixConfigMapName,
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

// SetApplicationConfig Set radixconfig to be used later by other steps, as well
// as deriving info from the config
func (info *PipelineInfo) SetApplicationConfig(applicationConfig *application.ApplicationConfig) {
	ra := applicationConfig.GetRadixApplicationConfig()
	info.RadixApplication = applicationConfig.GetRadixApplicationConfig()

	// Obtain metadata for rest of pipeline
	branchIsMapped, targetEnvironments := applicationConfig.IsThereAnythingToDeploy(info.PipelineArguments.Branch)

	// For deploy-only pipeline
	if info.IsDeployOnlyPipeline() {
		targetEnvironments[info.PipelineArguments.ToEnvironment] = true
		branchIsMapped = true
	}

	info.BranchIsMapped = branchIsMapped
	info.TargetEnvironments = targetEnvironments

	componentImages := getComponentImages(
		ra.GetName(),
		info.ContainerRegistry,
		info.PipelineArguments.ImageTag,
		ra.Spec.Components,
		ra.Spec.Jobs,
	)
	info.ComponentImages = componentImages
}

// IsDeployOnlyPipeline Determines if the pipeline is deploy-only
func (info *PipelineInfo) IsDeployOnlyPipeline() bool {
	return info.PipelineArguments.ToEnvironment != "" && info.PipelineArguments.FromEnvironment == ""
}

func getRadixComponentImageSources(components []v1.RadixComponent) []pipeline.ComponentImageSource {
	imageSources := make([]pipeline.ComponentImageSource, 0)

	for _, c := range components {
		s := pipeline.NewComponentImageSourceBuilder().
			WithSourceFunc(pipeline.RadixComponentSource(c)).
			Build()
		imageSources = append(imageSources, s)
	}

	return imageSources
}

func getRadixJobComponentImageSources(components []v1.RadixJobComponent) []pipeline.ComponentImageSource {
	imageSources := make([]pipeline.ComponentImageSource, 0)

	for _, c := range components {
		s := pipeline.NewComponentImageSourceBuilder().
			WithSourceFunc(pipeline.RadixJobComponentSource(c)).
			Build()
		imageSources = append(imageSources, s)
	}

	return imageSources
}

func getComponentImages(appName, containerRegistry, imageTag string, components []v1.RadixComponent, jobComponents []v1.RadixJobComponent) map[string]pipeline.ComponentImage {
	// Combine components and jobComponents

	componentSource := make([]pipeline.ComponentImageSource, 0)
	componentSource = append(componentSource, getRadixComponentImageSources(components)...)
	componentSource = append(componentSource, getRadixJobComponentImageSources(jobComponents)...)

	// First check if there are multiple components pointing to the same build context
	buildContextComponents := make(map[string][]componentType)

	// To ensure we can iterate over the map in the order
	// they were added
	buildContextKeys := make([]string, 0)

	for _, c := range componentSource {
		if c.Image != "" {
			// Using public image. Nothing to build
			continue
		}

		componentSource := getDockerfile(c.SourceFolder, c.DockerfileName)
		components := buildContextComponents[componentSource]
		if components == nil {
			components = make([]componentType, 0)
			buildContextKeys = append(buildContextKeys, componentSource)
		}

		components = append(components, componentType{c.Name, getContext(c.SourceFolder), getDockerfileName(c.DockerfileName)})
		buildContextComponents[componentSource] = components
	}

	componentImages := make(map[string]pipeline.ComponentImage)

	// Gather pre-built or public images
	for _, c := range componentSource {
		if c.Image != "" {
			componentImages[c.Name] = pipeline.ComponentImage{Build: false, Scan: false, ImageName: c.Image, ImagePath: c.Image}
		}
	}

	// Gather build containers
	numMultiComponentContainers := 0
	for _, key := range buildContextKeys {
		components := buildContextComponents[key]

		var imageName string

		if len(components) > 1 {
			log.Infof("Multiple components points to the same build context")
			imageName = multiComponentImageName

			if numMultiComponentContainers > 0 {
				// Start indexing them
				imageName = fmt.Sprintf("%s-%d", imageName, numMultiComponentContainers)
			}

			numMultiComponentContainers++
		} else {
			imageName = components[0].name
		}

		buildContainerName := fmt.Sprintf("build-%s", imageName)

		// A multi-component share context and dockerfile
		context := components[0].context
		dockerFile := components[0].dockerFileName

		// Set image back to component(s)
		for _, c := range components {
			componentImages[c.name] = pipeline.ComponentImage{
				ContainerName: buildContainerName,
				Context:       context,
				Dockerfile:    dockerFile,
				ImageName:     imageName,
				ImagePath:     utils.GetImagePath(containerRegistry, appName, imageName, imageTag),
				Build:         true,
				Scan:          true,
			}
		}
	}

	return componentImages
}

func getDockerfile(sourceFolder, dockerfileName string) string {
	context := getContext(sourceFolder)
	dockerfileName = getDockerfileName(dockerfileName)

	return fmt.Sprintf("%s%s", context, dockerfileName)
}

func getDockerfileName(name string) string {
	if name == "" {
		name = "Dockerfile"
	}

	return name
}

func getContext(sourceFolder string) string {
	sourceFolder = strings.Trim(sourceFolder, ".")
	sourceFolder = strings.Trim(sourceFolder, "/")
	if sourceFolder == "" {
		return fmt.Sprintf("%s/", git.Workspace)
	}
	return fmt.Sprintf("%s/%s/", git.Workspace, sourceFolder)
}
