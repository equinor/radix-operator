package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
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

const (
	appNameVariable  = "RADIX_APP"       // Required when repo is not cloned
	fileNameVariable = "RADIX_FILE_NAME" // Required when repo is cloned to point to location of the config
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
func prepareToRunPipeline() model.PipelineInfo {
	args := getArgs()

	appName := args[appNameVariable]
	fileName := args[fileNameVariable]

	// When we have deployment type pipelines
	// radix config is not cloned and should therefore
	// be retrived from cluster
	if fileName == "" && appName == "" {
		fileName, _ = filepath.Abs("./pipelines/testdata/radixconfig.yaml")
	}

	pipelineArgs := model.GetPipelineArgsFromArguments(args)
	client, radixClient, prometheusOperatorClient := utils.GetKubernetesClient()
	kubeUtil, _ := kube.New(client)

	pushHandler := pipe.Init(client, radixClient, prometheusOperatorClient)
	radixApplication, err := getRadixApplicationFromFileOrFromCluster(appName, fileName, radixClient)
	if err != nil {
		os.Exit(1)
	}

	radixRegistration, targetEnvironments, branchIsMapped, err := pushHandler.Prepare(radixApplication, pipelineArgs.Branch)
	if err != nil {
		os.Exit(1)
	}

	stepImplementations := initStepImplementations(radixRegistration,
		radixApplication,
		client,
		radixClient,
		kubeUtil,
		prometheusOperatorClient)

	pipelineDefinition, err := pipeline.GetPipelineFromName(pipelineArgs.PipelineType)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

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

func getRadixApplicationFromFileOrFromCluster(appName, fileName string, radixClient radixclient.Interface) (*v1.RadixApplication, error) {
	if fileName != "" {
		return loadConfigFromFile(fileName)
	} else {
		return radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	}
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
	registration *v1.RadixRegistration,
	applicationConfig *v1.RadixApplication,
	kubeclient kubernetes.Interface,
	radixclient radixclient.Interface,
	kubeutil *kube.Kube,
	prometheusOperatorClient monitoring.Interface) []model.Step {

	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep())
	stepImplementations = append(stepImplementations, steps.NewDeployStep())
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(registration, applicationConfig, kubeclient, radixclient, kubeutil, prometheusOperatorClient)
	}

	return stepImplementations
}
