package pipelines

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/applyconfig"
	"github.com/equinor/radix-operator/pipeline-runner/steps/build"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deploy"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deployconfig"
	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline"
	"github.com/equinor/radix-operator/pipeline-runner/steps/promote"
	"github.com/equinor/radix-operator/pipeline-runner/steps/runpipeline"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
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
func NewRunner(kubeclient kubernetes.Interface, radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusOperatorClient monitoring.Interface, secretsstorevclient secretsstorevclient.Interface, definition *pipeline.Definition, appName string) PipelineRunner {
	kubeUtil, _ := kube.New(kubeclient, radixclient, kedaClient, secretsstorevclient)
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
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to get RR for app %s. Error: %v", cli.appName, err)
		return err
	}

	stepImplementations := cli.initStepImplementations(ctx, radixRegistration)
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
	log.Ctx(ctx).Info().Msgf("Start pipeline %s for app %s", cli.pipelineInfo.Definition.Type, cli.appName)

	for _, step := range cli.pipelineInfo.Steps {
		logger := log.Ctx(ctx)
		ctx := logger.WithContext(ctx)

		err := step.Run(ctx, cli.pipelineInfo)
		if err != nil {
			logger.Error().Msg(step.ErrorMsg(err))
			return err
		}
		logger.Info().Msg(step.SucceededMsg())
		if cli.pipelineInfo.StopPipeline {
			logger.Info().Msgf("Pipeline is stopped: %s", cli.pipelineInfo.StopPipelineMessage)
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
		log.Ctx(ctx).Error().Err(err).Msgf("failed on tear-down deleting the config-map %s, ns: %s", cli.pipelineInfo.RadixConfigMapName, namespace)
	}

	if cli.pipelineInfo.IsPipelineType(v1.BuildDeploy) {
		err = cli.kubeUtil.DeleteConfigMap(ctx, namespace, cli.pipelineInfo.GitConfigMapName)
		if err != nil && !k8sErrors.IsNotFound(err) {
			log.Ctx(ctx).Error().Err(err).Msgf("failed on tear-down deleting the config-map %s, ns: %s", cli.pipelineInfo.GitConfigMapName, namespace)
		}
	}
}

func (cli *PipelineRunner) initStepImplementations(ctx context.Context, registration *v1.RadixRegistration) []model.Step {
	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, preparepipeline.NewPreparePipelinesStep(nil))
	stepImplementations = append(stepImplementations, applyconfig.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, build.NewBuildStep(nil))
	stepImplementations = append(stepImplementations, runpipeline.NewRunPipelinesStep(nil))
	stepImplementations = append(stepImplementations, deploy.NewDeployStep(watcher.NewNamespaceWatcherImpl(cli.kubeclient), watcher.NewRadixDeploymentWatcher(cli.radixclient, time.Minute*5)))
	stepImplementations = append(stepImplementations, deployconfig.NewDeployConfigStep(watcher.NewNamespaceWatcherImpl(cli.kubeclient), watcher.NewRadixDeploymentWatcher(cli.radixclient, time.Minute*5)))
	stepImplementations = append(stepImplementations, promote.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(ctx, cli.kubeclient, cli.radixclient, cli.kubeUtil, cli.prometheusOperatorClient, registration)
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
	log.Ctx(ctx).Debug().Msgf("Create the result ConfigMap %s in %s", configMap.GetName(), configMap.GetNamespace())
	_, err = cli.kubeUtil.CreateConfigMap(ctx, utils.GetAppNamespace(cli.appName), &configMap)
	return err
}
