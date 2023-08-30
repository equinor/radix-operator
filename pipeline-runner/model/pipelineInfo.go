package model

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils/conditions"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/maps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	multiComponentImageName = "multi-component"
)

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

	// Temporary data
	RadixConfigMapName string
	GitConfigMapName   string
	TargetEnvironments map[string]bool
	BranchIsMapped     bool
	// GitCommitHash is derived by inspecting HEAD commit after cloning user repository in prepare-pipelines step.
	// not to be confused with PipelineInfo.PipelineArguments.CommitID
	GitCommitHash string
	GitTags       string

	// Holds information on the images referred to by their respective components
	ComponentImages map[string]pipeline.ComponentImage
	// Prepare pipeline job build context
	PrepareBuildContext *PrepareBuildContext
	StopPipeline        bool
	StopPipelineMessage string

	// Promotion job git info
	SourceDeploymentGitCommitHash string
	SourceDeploymentGitBranch     string
}

// PipelineArguments Holds arguments for the pipeline
type PipelineArguments struct {
	PipelineType string
	JobName      string
	Branch       string
	// CommitID is sent from GitHub webhook. not to be confused with PipelineInfo.GitCommitHash
	CommitID        string
	ImageTag        string
	UseCache        bool
	PushImage       bool
	DeploymentName  string
	FromEnvironment string
	ToEnvironment   string

	RadixConfigFile string
	// Security context
	PodSecurityContext corev1.PodSecurityContext
	// Security context for image builder pods
	BuildKitPodSecurityContext corev1.PodSecurityContext

	ContainerSecurityContext corev1.SecurityContext
	// Images used for copying radix config/building
	TektonPipeline string
	// ImageBuilder Points to the image builder
	ImageBuilder string
	// BuildKitImageBuilder Points to the BuildKit compliant image builder
	BuildKitImageBuilder string
	// Used for tagging meta-information
	Clustertype string
	// RadixZone  The radix zone.
	RadixZone string
	// Clustername The name of the cluster
	Clustername string
	// ContainerRegistry The name of the container registry
	ContainerRegistry string
	// SubscriptionId Azure subscription ID
	SubscriptionId string
	// Used to indicate debugging session
	Debug bool
	// Image tag names for components: component-name:image-tag
	ImageTagNames map[string]string
	LogLevel      string
	AppName       string
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
	pipelineArguments *PipelineArguments,
	stepImplementations ...Step) (*PipelineInfo, error) {

	timestamp := time.Now().Format("20060102150405")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineArguments.JobName))
	radixConfigMapName := fmt.Sprintf("radix-config-2-map-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)
	gitConfigFileName := fmt.Sprintf("radix-git-information-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)

	podSecContext := securitycontext.Pod(securitycontext.WithPodFSGroup(defaults.SecurityContextFsGroup),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
	buildPodSecContext := securitycontext.Pod(
		securitycontext.WithPodFSGroup(defaults.SecurityContextFsGroup),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithPodRunAsNonRoot(conditions.BoolPtr(false)))
	containerSecContext := securitycontext.Container(securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsGroup(defaults.SecurityContextRunAsGroup),
		securitycontext.WithContainerRunAsUser(defaults.SecurityContextRunAsUser))

	pipelineArguments.ContainerSecurityContext = *containerSecContext
	pipelineArguments.PodSecurityContext = *podSecContext
	pipelineArguments.BuildKitPodSecurityContext = *buildPodSecContext

	stepImplementationsForType, err := getStepStepImplementationsFromType(pipelineType, stepImplementations...)
	if err != nil {
		return nil, err
	}

	return &PipelineInfo{
		Definition:         pipelineType,
		PipelineArguments:  *pipelineArguments,
		Steps:              stepImplementationsForType,
		RadixConfigMapName: radixConfigMapName,
		GitConfigMapName:   gitConfigFileName,
	}, nil
}

func getStepStepImplementationsFromType(pipelineType *pipeline.Definition, allStepImplementations ...Step) ([]Step, error) {
	stepImplementations := make([]Step, 0)

	for _, step := range pipelineType.Steps {
		stepImplementation := getStepImplementationForStepType(step, allStepImplementations)
		if stepImplementation == nil {
			return nil, fmt.Errorf("no step implementation found by type %s", stepImplementation)
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
		ra,
		info.PipelineArguments.ContainerRegistry,
		info.PipelineArguments.ImageTag,
		maps.GetKeysFromMap(targetEnvironments),
		info.PipelineArguments.ImageTagNames)
	info.ComponentImages = componentImages
}

// SetGitAttributes Set git attributes to be used later by other steps
func (info *PipelineInfo) SetGitAttributes(gitCommitHash, gitTags string) {
	info.GitCommitHash = gitCommitHash
	info.GitTags = gitTags
}

// IsDeployOnlyPipeline Determines if the pipeline is deploy-only
func (info *PipelineInfo) IsDeployOnlyPipeline() bool {
	return info.PipelineArguments.ToEnvironment != "" && info.PipelineArguments.FromEnvironment == ""
}

func getRadixComponentImageSources(ra *v1.RadixApplication, environments []string) []pipeline.ComponentImageSource {
	imageSources := make([]pipeline.ComponentImageSource, 0)

	for _, component := range ra.Spec.Components {
		if !component.GetEnabledForAnyEnvironment(environments) {
			continue
		}
		imageSource := pipeline.NewComponentImageSourceBuilder().
			WithSourceFunc(pipeline.RadixComponentSource(component)).
			Build()
		imageSources = append(imageSources, imageSource)
	}

	return imageSources
}

func getRadixJobComponentImageSources(ra *v1.RadixApplication, environments []string) []pipeline.ComponentImageSource {
	imageSources := make([]pipeline.ComponentImageSource, 0)

	for _, jobComponent := range ra.Spec.Jobs {
		if !jobComponent.GetEnabledForAnyEnvironment(environments) {
			continue
		}
		imageSource := pipeline.NewComponentImageSourceBuilder().
			WithSourceFunc(pipeline.RadixJobComponentSource(jobComponent)).
			Build()
		imageSources = append(imageSources, imageSource)
	}

	return imageSources
}

func getComponentImages(ra *v1.RadixApplication, containerRegistry, imageTag string, environments []string, imageTagNames map[string]string) map[string]pipeline.ComponentImage {
	// Combine components and jobComponents
	componentSource := make([]pipeline.ComponentImageSource, 0)
	componentSource = append(componentSource, getRadixComponentImageSources(ra, environments)...)
	componentSource = append(componentSource, getRadixJobComponentImageSources(ra, environments)...)

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
		if c.Image == "" {
			continue
		}
		componentImage := pipeline.ComponentImage{Build: false, ImageName: c.Image, ImagePath: c.Image}
		if imageTagNames != nil {
			componentImage.ImageTagName = imageTagNames[c.Name]
		}
		componentImages[c.Name] = componentImage
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
				ImagePath:     utils.GetImagePath(containerRegistry, ra.GetName(), imageName, imageTag),
				Build:         true,
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
