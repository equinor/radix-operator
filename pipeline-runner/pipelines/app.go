package onpush

import (
	"context"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonClient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// PipelineRunner Instance variables
type PipelineRunner struct {
	definition               *pipeline.Definition
	kubeclient               kubernetes.Interface
	kubeUtil                 *kube.Kube
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	tektonClient             versioned.Interface
	appName                  string
	pipelineInfo             *model.PipelineInfo
}

// InitRunner constructor
func InitRunner(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface, secretsstorevclient secretsstorevclient.Interface, tektonClient tektonClient.Interface, definition *pipeline.Definition, appName string) PipelineRunner {

	kubeUtil, _ := kube.New(kubeclient, radixclient, secretsstorevclient)
	handler := PipelineRunner{
		definition:               definition,
		kubeclient:               kubeclient,
		kubeUtil:                 kubeUtil,
		radixclient:              radixclient,
		tektonClient:             tektonClient,
		prometheusOperatorClient: prometheusOperatorClient,
		appName:                  appName,
	}

	return handler
}

// PrepareRun Runs preparations before build
func (cli *PipelineRunner) PrepareRun(pipelineArgs model.PipelineArguments) error {
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), cli.appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", cli.appName, err)
		return err
	}

	stepImplementations := initStepImplementations(cli.kubeclient, cli.kubeUtil, cli.radixclient,
		cli.prometheusOperatorClient, cli.tektonClient, radixRegistration)
	cli.pipelineInfo, err = model.InitPipeline(
		cli.definition,
		pipelineArgs,
		stepImplementations...)

	if err != nil {
		return err
	}

	containerRegistry, err := cli.kubeUtil.GetContainerRegistry()
	if err != nil {
		return err
	}
	subscriptionId, err := cli.kubeUtil.GetSubscriptionId()
	if err != nil {
		return err
	}

	cli.pipelineInfo.ContainerRegistry = containerRegistry
	cli.pipelineInfo.SubscriptionId = subscriptionId
	return nil
}

// Run runs through the steps in the defined pipeline
func (cli *PipelineRunner) Run() error {
	log.Infof("Start pipeline %s for app %s", cli.pipelineInfo.Definition.Type, cli.appName)

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

// TearDown performs any needed cleanup
func (cli *PipelineRunner) TearDown() {
	namespace := utils.GetAppNamespace(cli.appName)
	configMapName := cli.pipelineInfo.RadixConfigMapName
	err := cli.kubeclient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), configMapName, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed on tear-down deleting the config-map %s, ns: %s. %v", configMapName, namespace, err)
	}
}

func initStepImplementations(kubeclient kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface, tektonClient versioned.Interface, registration *v1.RadixRegistration) []model.Step {

	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, steps.NewCopyConfigToMapStep())
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep())
	stepImplementations = append(stepImplementations, steps.NewCustomPipelineStep())
	stepImplementations = append(stepImplementations, steps.NewScanImageStep())
	stepImplementations = append(stepImplementations, steps.NewDeployStep(kube.NewNamespaceWatcherImpl(kubeclient)))
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(kubeclient, radixclient, kubeUtil, prometheusOperatorClient, tektonClient, registration)
	}

	return stepImplementations
}
