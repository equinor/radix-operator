package onpush

import (
	"fmt"
	"strings"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const multiComponentImageName = "multi-component"

type componentType struct {
	name           string
	context        string
	dockerFileName string
}

// PipelineRunner Instance variables
type PipelineRunner struct {
	definfition              *pipeline.Definition
	kubeclient               kubernetes.Interface
	kubeUtil                 *kube.Kube
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	radixApplication         *v1.RadixApplication
	pipelineInfo             *model.PipelineInfo
}

// InitRunner constructor
func InitRunner(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface,
	definfition *pipeline.Definition, radixApplication *v1.RadixApplication) PipelineRunner {

	kubeUtil, _ := kube.New(kubeclient, radixclient)
	handler := PipelineRunner{
		definfition:              definfition,
		kubeclient:               kubeclient,
		kubeUtil:                 kubeUtil,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		radixApplication:         radixApplication,
	}

	return handler
}

// PrepareRun Runs preparations before build
func (cli *PipelineRunner) PrepareRun(pipelineArgs model.PipelineArguments) error {
	if validate.RAContainsOldPublic(cli.radixApplication) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.radixclient, cli.radixApplication)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		return errors.Concat(errs)
	}

	appName := cli.radixApplication.GetName()
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", appName, err)
		return err
	}

	applicationConfig, err := application.NewApplicationConfig(cli.kubeclient, cli.kubeUtil,
		cli.radixclient, radixRegistration, cli.radixApplication)
	if err != nil {
		return err
	}

	containerRegistry, err := cli.kubeUtil.GetContainerRegistry()
	if err != nil {
		return err
	}

	branchIsMapped, targetEnvironments := applicationConfig.IsBranchMappedToEnvironment(pipelineArgs.Branch)

	stepImplementations := initStepImplementations(cli.kubeclient, cli.kubeUtil, cli.radixclient, cli.prometheusOperatorClient, radixRegistration, cli.radixApplication)
	cli.pipelineInfo, err = model.InitPipeline(
		cli.definfition,
		targetEnvironments,
		branchIsMapped,
		pipelineArgs,
		stepImplementations...)

	componentImages := getComponentImages(appName, containerRegistry, cli.pipelineInfo.PipelineArguments.ImageTag, cli.radixApplication.Spec.Components)
	cli.pipelineInfo.ComponentImages = componentImages

	if err != nil {
		return err
	}

	return nil
}

// Run runs throught the steps in the defined pipeline
func (cli *PipelineRunner) Run() error {
	appName := cli.radixApplication.GetName()

	log.Infof("Start pipeline %s for app %s", cli.pipelineInfo.Definition.Type, appName)

	for _, step := range cli.pipelineInfo.Steps {
		err := step.Run(cli.pipelineInfo)
		if err != nil {
			log.Errorf(step.ErrorMsg(err))
			return err
		}
		log.Infof(step.SucceededMsg())
	}
	return nil
}

func initStepImplementations(
	kubeclient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusOperatorClient monitoring.Interface,
	registration *v1.RadixRegistration,
	radixApplication *v1.RadixApplication) []model.Step {

	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, steps.NewCopyConfigToMapStep())
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep())
	stepImplementations = append(stepImplementations, steps.NewScanImageStep())
	stepImplementations = append(stepImplementations, steps.NewDeployStep(kube.NewNamespaceWatcherImpl(kubeclient)))
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(kubeclient, radixclient, kubeUtil, prometheusOperatorClient, registration, radixApplication)
	}

	return stepImplementations
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

// TODO: The following functions are duplicate of step.Build private function. Either, remove one of them
// or move them to a common area
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
