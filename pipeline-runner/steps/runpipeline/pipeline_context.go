package runpipeline

import (
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/wait"
	ownerreferences "github.com/equinor/radix-operator/pipeline-runner/utils/owner_references"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Context of the pipeline
type Context interface {
	// RunPipelinesJob un the job, which creates Tekton PipelineRun-s
	RunPipelinesJob(*model.PipelineInfo) error
}

type pipelineContext struct {
	tektonClient       tektonclient.Interface
	targetEnvironments []string
	hash               string
	ownerReference     *metav1.OwnerReference
	waiter             wait.PipelineRunsCompletionWaiter
	pipelineInfo       *model.PipelineInfo
}

// GetEnvVars Gets build env vars
func (step *RunPipelinesStepImplementation) GetEnvVars(envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	step.setPipelineRunParamsFromBuild(envVarsMap)
	step.setPipelineRunParamsFromEnvironmentBuilds(envName, envVarsMap)
	return envVarsMap
}

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromBuild(envVarsMap radixv1.EnvVarsMap) {
	ra := step.pipelineInfo.GetRadixApplication()
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

func (step *RunPipelinesStepImplementation) setPipelineRunParamsFromEnvironmentBuilds(targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range step.pipelineInfo.GetRadixApplication().Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

type NewPipelineContextOption func(step *RunPipelinesStepImplementation)

// NewPipelineContext Create new NewPipelineContext instance
func NewPipelineContext(tektonClient tektonclient.Interface, pipelineInfo *model.PipelineInfo, targetEnvironments []string, options ...NewPipelineContextOption) Context {
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	step := &pipelineContext{
		tektonClient:       tektonClient,
		pipelineInfo:       pipelineInfo,
		hash:               strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName)),
		ownerReference:     ownerReference,
		waiter:             wait.NewPipelineRunsCompletionWaiter(tektonClient),
		targetEnvironments: targetEnvironments,
	}

	for _, option := range options {
		option(step)
	}
	return step
}

// WithPipelineRunsWaiter Set pipeline runs waiter
func WithPipelineRunsWaiter(waiter wait.PipelineRunsCompletionWaiter) NewPipelineContextOption {
	return func(step *RunPipelinesStepImplementation) {
		step.waiter = waiter
	}
}
