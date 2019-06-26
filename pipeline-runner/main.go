package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	pipe "github.com/equinor/radix-operator/pipeline-runner/pipelines"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Requirements to run, pipeline must have:
// - access to read RR of the app mention in "RADIX_FILE_NAME"
// - access to create Jobs in "app" namespace it runs under
// - access to create RD in all namespaces
// - access to create new namespaces
// - a secret git-ssh-keys containing deployment key to git repo provided in RR
// - a secret radix-docker with credentials to access our private ACR
func main() {
	pipelineInfo := prepareToRunPipeline()

	err := pipe.Run(pipelineInfo)
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// runs os.Exit(1) if error
func prepareToRunPipeline() *model.PipelineInfo {
	args := getArgs()

	// Required when repo is not cloned
	appName := args["RADIX_APP"]

	// Required when repo is cloned to point to location of the config
	fileName := args["RADIX_FILE_NAME"]

	pipelineArgs := model.GetPipelineArgsFromArguments(args)
	client, radixClient, prometheusOperatorClient := utils.GetKubernetesClient()

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	pushHandler := pipe.Init(client, radixClient, prometheusOperatorClient)
	radixApplication, err := getRadixApplicationFromFileOrFromCluster(pipelineDefinition, appName, fileName, radixClient)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	radixRegistration, err := pushHandler.Prepare(radixApplication)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	applicationConfig, err := application.NewApplicationConfig(client, radixClient, radixRegistration, radixApplication)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	branchIsMapped, targetEnvironments := applicationConfig.IsBranchMappedToEnvironment(pipelineArgs.Branch)

	stepImplementations := initStepImplementations(client, radixClient, prometheusOperatorClient)
	pipelineInfo, err := model.InitPipeline(
		pipelineDefinition,
		radixRegistration,
		radixApplication,
		targetEnvironments,
		branchIsMapped,
		pipelineArgs,
		stepImplementations...)

	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	return pipelineInfo
}

func getRadixApplicationFromFileOrFromCluster(pipelineDefinition *pipeline.Definition, appName, fileName string, radixClient radixclient.Interface) (*v1.RadixApplication, error) {
	// When we have deployment-only type pipelines (currently only promote)
	// radix config is not cloned and should therefore
	// be retrived from cluster
	if pipelineDefinition.Name == pipeline.Promote {
		if appName == "" {
			return nil, fmt.Errorf("App name is a required parameter for %s pipelines", pipelineDefinition.Name)
		}

		return radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	}

	if fileName == "" {
		return nil, fmt.Errorf("Filename is a required parameter for %s pipelines", pipelineDefinition.Name)
	}

	filePath, _ := filepath.Abs(fileName)
	return loadConfigFromFile(filePath)
}

// LoadConfigFromFile loads radix config from appFileName
func loadConfigFromFile(appFileName string) (*v1.RadixApplication, error) {
	radixApplication, err := utils.GetRadixApplication(appFileName)
	if err != nil {
		log.Errorf("Failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return nil, err
	}

	return radixApplication, nil
}

func getArgs() map[string]string {
	argsWithoutProg := os.Args[1:]
	args := map[string]string{}
	for _, arg := range argsWithoutProg {
		keyValue := strings.Split(arg, "=")
		key := keyValue[0]
		value := keyValue[1]
		args[key] = value
	}
	return args
}

func initStepImplementations(
	kubeclient kubernetes.Interface,
	radixclient radixclient.Interface,
	prometheusOperatorClient monitoring.Interface) []model.Step {

	kubeUtil, _ := kube.New(kubeclient)
	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep())
	stepImplementations = append(stepImplementations, steps.NewDeployStep())
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(kubeclient, radixclient, kubeUtil, prometheusOperatorClient)
	}

	return stepImplementations
}
