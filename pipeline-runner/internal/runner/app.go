package runner

import (
	"context"
	"errors"
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
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// PipelineRunner Instance variables
type PipelineRunner struct {
	definition    *pipeline.Definition
	kubeClient    kubernetes.Interface
	kubeUtil      *kube.Kube
	radixClient   radixclient.Interface
	tektonClient  tektonclient.Interface
	dynamicClient client.Client
	appName       string
	pipelineInfo  *model.PipelineInfo
}

// NewRunner constructor
func NewRunner(kubeClient kubernetes.Interface, radixClient radixclient.Interface, kedaClient kedav2.Interface, dynamicClient client.Client, secretsStoreClient secretsstoreclient.Interface, tektonClient tektonclient.Interface, definition *pipeline.Definition, appName string) PipelineRunner {
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretsStoreClient)
	handler := PipelineRunner{
		definition:    definition,
		kubeClient:    kubeClient,
		kubeUtil:      kubeUtil,
		radixClient:   radixClient,
		tektonClient:  tektonClient,
		dynamicClient: dynamicClient,
		appName:       appName,
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

	return err
}

// Run runs through the steps in the defined pipeline
func (cli *PipelineRunner) Run(ctx context.Context) error {
	printPipelineDescription(ctx, cli.pipelineInfo)
	cli.UpdateStatus(ctx, v1.JobRunning)
	for _, step := range cli.pipelineInfo.Steps {
		logger := log.Ctx(ctx)
		ctx := logger.WithContext(ctx)

		if err := step.Run(ctx, cli.pipelineInfo); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info().Msg("Pipeline run is canceled")
				cli.UpdateStatus(ctx, v1.JobStopped)
				return nil
			}

			logger.Error().Msg(step.ErrorMsg(err))
			cli.UpdateStatus(ctx, v1.JobFailed)
			return err
		}

		logger.Info().Msg(step.SucceededMsg())

		if cli.pipelineInfo.StopPipeline {
			logger.Info().Msgf("Pipeline is stopped: %s", cli.pipelineInfo.StopPipelineMessage)
			cli.UpdateStatus(ctx, v1.JobStoppedNoChanges)
			return nil
		}
	}
	cli.UpdateStatus(ctx, v1.JobSucceeded)
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
			Init(ctx, cli.kubeClient, cli.radixClient, cli.dynamicClient, cli.tektonClient, registration)
	}

	return stepImplementations
}

func printPipelineDescription(ctx context.Context, pipelineInfo *model.PipelineInfo) {
	appName := pipelineInfo.GetAppName()
	switch pipelineInfo.GetRadixPipelineType() {
	case v1.ApplyConfig:
		log.Ctx(ctx).Info().Msgf("Apply radixconfig for application %s", appName)
	case v1.BuildDeploy:
		log.Ctx(ctx).Info().Msgf("Build and deploy application %s from %s %s", appName, pipelineInfo.GetGitRefTypeOrDefault(), pipelineInfo.GetGitRefOrDefault())
	case v1.Build:
		log.Ctx(ctx).Info().Msgf("Build application %s from %s %s", appName, pipelineInfo.GetGitRefTypeOrDefault(), pipelineInfo.GetGitRefOrDefault())
	case v1.Deploy:
		log.Ctx(ctx).Info().Msgf("Deploy application %s to environment %s", appName, pipelineInfo.GetRadixDeployToEnvironment())
	case v1.Promote:
		log.Ctx(ctx).Info().Msgf("Promote deployment %s of application %s from environment %s to %s", pipelineInfo.GetRadixPromoteDeployment(), appName, pipelineInfo.GetRadixPromoteFromEnvironment(), pipelineInfo.GetRadixDeployToEnvironment())
	}
}

// UpdateStatus updates the status of the pipeline job. It ignores any cancelled context, and creates its own 30s deadline
func (cli *PipelineRunner) UpdateStatus(ctx context.Context, condition v1.RadixJobCondition) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rj := &v1.RadixJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cli.pipelineInfo.PipelineArguments.JobName,
				Namespace: utils.GetAppNamespace(cli.appName),
			},
		}

		err := cli.dynamicClient.Get(ctx, client.ObjectKeyFromObject(rj), rj)
		if err != nil {
			return err
		}

		if rj.Status.PipelineRunStatus == nil {
			rj.Status.PipelineRunStatus = &v1.RadixJobPipelineRunStatus{}
		}

		rj.Status.PipelineRunStatus.UsedBuildCache = cli.pipelineInfo.IsUsingBuildCache()
		rj.Status.PipelineRunStatus.UsedBuildKit = cli.pipelineInfo.IsUsingBuildKit()
		rj.Status.PipelineRunStatus.Status = condition

		return cli.dynamicClient.Status().Update(ctx, rj)
	})

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("Failed to update status of pipeline job %s", cli.pipelineInfo.PipelineArguments.JobName)
	}
}
