package onpush

import (
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils/errors"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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

	branchIsMapped, targetEnvironments := applicationConfig.IsBranchMappedToEnvironment(pipelineArgs.Branch)

	stepImplementations := initStepImplementations(cli.kubeclient, cli.kubeUtil, cli.radixclient, cli.prometheusOperatorClient, radixRegistration, cli.radixApplication)
	cli.pipelineInfo, err = model.InitPipeline(
		cli.definfition,
		targetEnvironments,
		branchIsMapped,
		pipelineArgs,
		stepImplementations...)

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
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep())
	stepImplementations = append(stepImplementations, steps.NewDeployStep(kube.NewNamespaceWatcherImpl(kubeclient)))
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(kubeclient, radixclient, kubeUtil, prometheusOperatorClient, registration, radixApplication)
	}

	return stepImplementations
}
