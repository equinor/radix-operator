package runpipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
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
	waiter wait.PipelineRunsCompletionWaiter
}

// NewRunPipelinesStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewRunPipelinesStep(options ...RunPipelinesStepImplementationOption) model.Step {
	step := &RunPipelinesStepImplementation{
		stepType: pipeline.RunPipelinesStep,
	}
	for _, option := range options {
		option(step)
	}
	return step
}

func (step *RunPipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
	if step.waiter == nil {
		step.waiter = wait.NewPipelineRunsCompletionWaiter(tektonClient)
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
	if pipelineInfo.BuildContext != nil && len(pipelineInfo.BuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Ctx(ctx).Info().Msg("There is no configured sub-pipelines. Skip the step.")
		return nil
	}
	targetEnvironments, ignoredForWebhookEnvs, err := internal.GetPipelineTargetEnvironments(ctx, pipelineInfo)
	if err != nil {
		return err
	}
	if len(ignoredForWebhookEnvs) > 0 {
		log.Ctx(ctx).Info().Msgf("Run sub-pipelines for following environment(s) are ignored to be triggered by the webhook: %s", strings.Join(ignoredForWebhookEnvs, ", "))
	}
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash
	appName := step.GetAppName()
	log.Ctx(ctx).Info().Msgf("Run pipelines app %s for branch %s and commit %s", appName, branch, commitID)
	return step.RunPipelinesJob(pipelineInfo, targetEnvironments)
}

// GetEnvVars Gets build env vars
func (step *RunPipelinesStepImplementation) GetEnvVars(pipelineInfo *model.PipelineInfo, envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	step.setPipelineRunParamsFromBuild(pipelineInfo, envVarsMap)
	step.setPipelineRunParamsFromEnvironmentBuilds(pipelineInfo, envName, envVarsMap)
	return envVarsMap
}

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromBuild(pipelineInfo *model.PipelineInfo, envVarsMap radixv1.EnvVarsMap) {
	ra := pipelineInfo.GetRadixApplication()
	if ra.Spec.Build == nil {
		return
	}
	setBuildIdentity(envVarsMap, ra.Spec.Build.SubPipeline)
	setBuildVariables(envVarsMap, ra.Spec.Build.SubPipeline, ra.Spec.Build.Variables)
}

func setBuildVariables(envVarsMap radixv1.EnvVarsMap, subPipeline *radixv1.SubPipeline, variables radixv1.EnvVarsMap) {
	if subPipeline != nil {
		setVariablesToEnvVarsMap(envVarsMap, subPipeline.Variables) // sub-pipeline variables have higher priority over build variables
		return
	}
	setVariablesToEnvVarsMap(envVarsMap, variables) // keep for backward compatibility
}

func setVariablesToEnvVarsMap(envVarsMap radixv1.EnvVarsMap, variables radixv1.EnvVarsMap) {
	for name, envVar := range variables {
		envVarsMap[name] = envVar
	}
}

func setBuildIdentity(envVarsMap radixv1.EnvVarsMap, subPipeline *radixv1.SubPipeline) {
	if subPipeline != nil {
		setIdentityToEnvVarsMap(envVarsMap, subPipeline.Identity)
	}
}

func setIdentityToEnvVarsMap(envVarsMap radixv1.EnvVarsMap, identity *radixv1.Identity) {
	if identity == nil || identity.Azure == nil {
		return
	}
	if len(identity.Azure.ClientId) > 0 {
		envVarsMap[defaults.AzureClientIdEnvironmentVariable] = identity.Azure.ClientId // if build env-var or build environment env-var have this variable explicitly set, it will override this identity set env-var
	} else {
		delete(envVarsMap, defaults.AzureClientIdEnvironmentVariable)
	}
}

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromEnvironmentBuilds(pipelineInfo *model.PipelineInfo, targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range pipelineInfo.GetRadixApplication().Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

type RunPipelinesStepImplementationOption func(step *RunPipelinesStepImplementation)

// WithPipelineRunsWaiter Set pipeline runs waiter
func WithPipelineRunsWaiter(waiter wait.PipelineRunsCompletionWaiter) RunPipelinesStepImplementationOption {
	return func(step *RunPipelinesStepImplementation) {
		step.waiter = waiter
	}
}
