package steps

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v1"
)

const multiComponentImageName = "multi-component"

type componentType struct {
	name           string
	context        string
	dockerFileName string
}

// ApplyConfigStepImplementation Step to apply RA
type ApplyConfigStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewApplyConfigStep Constructor
func NewApplyConfigStep() model.Step {
	return &ApplyConfigStepImplementation{
		stepType: pipeline.ApplyConfigStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ApplyConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *ApplyConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Applied config for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *ApplyConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to apply config for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *ApplyConfigStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	// Get radix application from config map
	namespace := utils.GetAppNamespace(cli.GetAppName())
	configMap, err := cli.GetKubeutil().GetConfigMap(namespace, pipelineInfo.RadixConfigMapName)
	if err != nil {
		return err
	}

	ra := &v1.RadixApplication{}
	err = yaml.Unmarshal([]byte(configMap.Data["content"]), ra)

	// Validate RA
	if validate.RAContainsOldPublic(ra) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.GetRadixclient(), ra)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		return errors.Concat(errs)
	}

	// Apply RA to cluster
	applicationConfig, err := application.NewApplicationConfig(cli.GetKubeclient(), cli.GetKubeutil(),
		cli.GetRadixclient(), cli.GetRegistration(), ra)
	if err != nil {
		return err
	}

	applicationConfig, err = application.NewApplicationConfig(cli.GetKubeclient(), cli.GetKubeutil(), cli.GetRadixclient(), cli.GetRegistration(), cli.GetApplicationConfig())
	if err != nil {
		return err
	}

	err = applicationConfig.ApplyConfigToApplicationNamespace()
	if err != nil {
		return err
	}

	// Obtain metadata for rest of pipeline
	branchIsMapped, targetEnvironments := applicationConfig.IsBranchMappedToEnvironment(pipelineInfo.PipelineArguments.Branch)
	pipelineInfo.BranchIsMapped = branchIsMapped
	pipelineInfo.TargetEnvironments = targetEnvironments

	containerRegistry, err := cli.GetKubeutil().GetContainerRegistry()
	if err != nil {
		return err
	}

	componentImages := getComponentImages(cli.GetAppName(), containerRegistry, pipelineInfo.PipelineArguments.ImageTag, ra.Spec.Components)
	pipelineInfo.ComponentImages = componentImages

	return nil
}

func getComponentImages(appName, containerRegistry, imageTag string, components []v1.RadixComponent) map[string]pipeline.ComponentImage {
	// First check if there are multiple components pointing to the same build context
	buildContextComponents := make(map[string][]componentType)

	// To ensure we can iterate over the map in the order
	// they were added
	buildContextKeys := make([]string, 0)

	for _, c := range components {
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
	for _, c := range components {
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
