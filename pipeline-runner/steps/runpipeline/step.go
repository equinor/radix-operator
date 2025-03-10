package runpipeline

import (
	"context"
	"fmt"

	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// RunPipelinesStepImplementation Step to run Tekton pipelines
type RunPipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	jobWaiter internalwait.JobCompletionWaiter
}

// NewRunPipelinesStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewRunPipelinesStep(jobWaiter internalwait.JobCompletionWaiter) model.Step {
	return &RunPipelinesStepImplementation{
		stepType:  pipeline.RunPipelinesStep,
		jobWaiter: jobWaiter,
	}
}

func (step *RunPipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
	if step.jobWaiter == nil {
		step.jobWaiter = internalwait.NewJobCompletionWaiter(ctx, kubeClient)
	}
}

// ImplementationForType Override of default step method
func (step *RunPipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *RunPipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: run pipelines step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *RunPipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed run pipelines for the application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *RunPipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.PrepareBuildContext != nil && len(pipelineInfo.PrepareBuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Ctx(ctx).Info().Msg("There is no configured sub-pipelines. Skip the step.")
		return nil
	}
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash
	appName := step.GetAppName()
	log.Ctx(ctx).Info().Msgf("Run pipelines app %s for branch %s and commit %s", appName, branch, commitID)

	pipelineCtx := NewPipelineContext(cli.GetKubeClient(), cli.GetRadixClient(), cli.GetTektonClient(), pipelineInfo)
	return pipelineCtx.RunPipelinesJob()
}
