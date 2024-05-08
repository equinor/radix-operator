package pipelines

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	"sigs.k8s.io/yaml"
)

// PipelineRunner Instance variables
type PipelineRunner struct {
	definition               *pipeline.Definition
	kubeclient               kubernetes.Interface
	kubeUtil                 *kube.Kube
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	appName                  string
	pipelineInfo             *model.PipelineInfo
}

// NewRunner constructor
func NewRunner(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface, secretsstorevclient secretsstorevclient.Interface, definition *pipeline.Definition, appName string) PipelineRunner {
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretsstorevclient)
	handler := PipelineRunner{
		definition:               definition,
		kubeclient:               kubeclient,
		kubeUtil:                 kubeUtil,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		appName:                  appName,
	}

	return handler
}

// PrepareRun Runs preparations before build
func (cli *PipelineRunner) PrepareRun(ctx context.Context, pipelineArgs *model.PipelineArguments) error {
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations().Get(ctx, cli.appName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get RR for app %s. Error: %v", cli.appName, err)
		return err
	}

	stepImplementations := cli.initStepImplementations(radixRegistration)
	cli.pipelineInfo, err = model.InitPipeline(
		cli.definition,
		pipelineArgs,
		stepImplementations...)

	if err != nil {
		return err
	}
	return nil
}

// Run runs through the steps in the defined pipeline
func (cli *PipelineRunner) Run(ctx context.Context) error {
	log.Info().Msgf("Start pipeline %s for app %s", cli.pipelineInfo.Definition.Type, cli.appName)

	for _, step := range cli.pipelineInfo.Steps {
		err := step.Run(ctx, cli.pipelineInfo)
		if err != nil {
			log.Error().Msg(step.ErrorMsg(err))
			return err
		}
		log.Info().Msg(step.SucceededMsg())
		if cli.pipelineInfo.StopPipeline {
			log.Info().Msgf("Pipeline is stopped: %s", cli.pipelineInfo.StopPipelineMessage)
			break
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// TearDown performs any needed cleanup
func (cli *PipelineRunner) TearDown(ctx context.Context) {
	namespace := utils.GetAppNamespace(cli.appName)

	err := cli.kubeUtil.DeleteConfigMap(ctx, namespace, cli.pipelineInfo.RadixConfigMapName)
	if err != nil && !k8sErrors.IsNotFound(err) {
		log.Error().Err(err).Msgf("failed on tear-down deleting the config-map %s, ns: %s", cli.pipelineInfo.RadixConfigMapName, namespace)
	}

	if cli.pipelineInfo.IsPipelineType(v1.BuildDeploy) {
		err = cli.kubeUtil.DeleteConfigMap(ctx, namespace, cli.pipelineInfo.GitConfigMapName)
		if err != nil && !k8sErrors.IsNotFound(err) {
			log.Error().Err(err).Msgf("failed on tear-down deleting the config-map %s, ns: %s", cli.pipelineInfo.GitConfigMapName, namespace)
		}
	}
}

func (cli *PipelineRunner) initStepImplementations(registration *v1.RadixRegistration) []model.Step {
	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, steps.NewPreparePipelinesStep(nil))
	stepImplementations = append(stepImplementations, steps.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, steps.NewBuildStep(nil))
	stepImplementations = append(stepImplementations, steps.NewRunPipelinesStep(nil))
	stepImplementations = append(stepImplementations, steps.NewDeployStep(kube.NewNamespaceWatcherImpl(cli.kubeclient), watcher.NewRadixDeploymentWatcher(cli.radixclient, time.Minute*5)))
	stepImplementations = append(stepImplementations, steps.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(cli.kubeclient, cli.radixclient, cli.kubeUtil, cli.prometheusOperatorClient, registration)
	}

	return stepImplementations
}

func (cli *PipelineRunner) CreateResultConfigMap(ctx context.Context) error {
	result := v1.RadixJobResult{}
	if cli.pipelineInfo.StopPipeline {
		result.Result = v1.RadixJobResultStoppedNoChanges
		result.Message = cli.pipelineInfo.StopPipelineMessage
	}
	resultContent, err := yaml.Marshal(&result)
	if err != nil {
		return err
	}
	pipelineJobName := cli.pipelineInfo.PipelineArguments.JobName

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineJobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:       pipelineJobName,
				kube.RadixConfigMapTypeLabel: string(kube.RadixPipelineResultConfigMap),
			},
		},
		Data: map[string]string{jobs.ResultContent: string(resultContent)},
	}
	log.Debug().Msgf("Create the result ConfigMap %s in %s", configMap.GetName(), configMap.GetNamespace())
	_, err = cli.kubeUtil.CreateConfigMap(ctx, utils.GetAppNamespace(cli.appName), &configMap)
	return err
}
