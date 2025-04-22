package runner

import (
	"context"
	"fmt"
	"strings"
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
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	secretsstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	"sigs.k8s.io/yaml"
)

// PipelineRunner Instance variables
type PipelineRunner struct {
	definition               *pipeline.Definition
	kubeClient               kubernetes.Interface
	kubeUtil                 *kube.Kube
	radixClient              radixclient.Interface
	tektonClient             tektonclient.Interface
	prometheusOperatorClient monitoring.Interface
	appName                  string
	pipelineInfo             *model.PipelineInfo
}

// NewRunner constructor
func NewRunner(kubeClient kubernetes.Interface, radixClient radixclient.Interface, kedaClient kedav2.Interface, prometheusOperatorClient monitoring.Interface, secretsStoreClient secretsstoreclient.Interface, tektonClient tektonclient.Interface, definition *pipeline.Definition, appName string) PipelineRunner {
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretsStoreClient)
	handler := PipelineRunner{
		definition:               definition,
		kubeClient:               kubeClient,
		kubeUtil:                 kubeUtil,
		radixClient:              radixClient,
		tektonClient:             tektonClient,
		prometheusOperatorClient: prometheusOperatorClient,
		appName:                  appName,
	}
	return handler
}

// PrepareRun Runs preparations before build
func (cli *PipelineRunner) PrepareRun(ctx context.Context, pipelineArgs *model.PipelineArguments) error {
	radixRegistration, err := cli.radixClient.RadixV1().RadixRegistrations().Get(ctx, cli.appName, metav1.GetOptions{})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to get RR for app %s. Error: %v", cli.appName, err)
		return err
	}

	stepImplementations := cli.initStepImplementations(ctx, radixRegistration)
	cli.pipelineInfo, err = model.InitPipeline(cli.definition, pipelineArgs, stepImplementations...)
	if err != nil {
		return err
	}
	cli.pipelineInfo.RadixRegistration = radixRegistration
	return err
}

// Run runs through the steps in the defined pipeline
func (cli *PipelineRunner) Run(ctx context.Context) error {
	printPipelineDescription(ctx, cli.pipelineInfo)
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

func (cli *PipelineRunner) initStepImplementations(ctx context.Context, registration *v1.RadixRegistration) []model.Step {
	stepImplementations := make([]model.Step, 0)
	stepImplementations = append(stepImplementations, preparepipeline.NewPreparePipelinesStep())
	stepImplementations = append(stepImplementations, applyconfig.NewApplyConfigStep())
	stepImplementations = append(stepImplementations, build.NewBuildStep(nil))
	stepImplementations = append(stepImplementations, runpipeline.NewRunPipelinesStep())
	stepImplementations = append(stepImplementations, deploy.NewDeployStep(watcher.NewNamespaceWatcherImpl(cli.kubeClient), watcher.NewRadixDeploymentWatcher(cli.radixClient, time.Minute*5)))
	stepImplementations = append(stepImplementations, deployconfig.NewDeployConfigStep(watcher.NewRadixDeploymentWatcher(cli.radixClient, time.Minute*5)))
	stepImplementations = append(stepImplementations, promote.NewPromoteStep())

	for _, stepImplementation := range stepImplementations {
		stepImplementation.
			Init(ctx, cli.kubeClient, cli.radixClient, cli.kubeUtil, cli.prometheusOperatorClient, cli.tektonClient, registration)
	}

	return stepImplementations
}

// CreateResultConfigMap Creates a ConfigMap with the result of the pipeline job
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

	pipelineJobName := strings.ToLower(fmt.Sprintf("%s-%s", cli.pipelineInfo.PipelineArguments.JobName, utils.RandString(5)))
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

func printPipelineDescription(ctx context.Context, pipelineInfo *model.PipelineInfo) {
	appName := pipelineInfo.GetAppName()
	branch := pipelineInfo.GetBranch()
	switch pipelineInfo.GetRadixPipelineType() {
	case radixv1.ApplyConfig:
		log.Ctx(ctx).Info().Msgf("Apply the radixconfig for the application %s", appName)
	case radixv1.BuildDeploy:
		log.Ctx(ctx).Info().Msgf("Build and deploy the application %s from the branch %s", appName, branch)
	case radixv1.Build:
		log.Ctx(ctx).Info().Msgf("Build the application %s from the branch %s", appName, branch)
	case radixv1.Deploy:
		log.Ctx(ctx).Info().Msgf("Deploy the application %s to the environment %s", appName, pipelineInfo.GetRadixDeployToEnvironment())
	case radixv1.Promote:
		log.Ctx(ctx).Info().Msgf("Promote the deployment %s of the application %s from the environment %s to %s", pipelineInfo.GetRadixPromoteDeployment(), appName, pipelineInfo.GetRadixPromoteFromEnvironment(), pipelineInfo.GetRadixDeployToEnvironment())
	}
}
