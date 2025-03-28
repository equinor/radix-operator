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
	RunPipelinesJob() error
	// GetPipelineInfo Get pipeline info
	GetPipelineInfo() *model.PipelineInfo
	// GetHash Hash, common for all pipeline Kubernetes object names
	GetHash() string
	// GetTektonClient Tekton client
	GetTektonClient() tektonclient.Interface
	// GetRadixApplication Gets the RadixApplication, loaded from the config-map
	GetRadixApplication() *radixv1.RadixApplication
	// GetPipelineRunsWaiter Returns a waiter that returns when all pipelineruns have completed
	GetPipelineRunsWaiter() wait.PipelineRunsCompletionWaiter
	// GetEnvVars Gets build env vars
	GetEnvVars(envName string) radixv1.EnvVarsMap
	// SetPipelineTargetEnvironments Set target environments for the pipeline job
	SetPipelineTargetEnvironments(environments []string)
}

type pipelineContext struct {
	tektonClient       tektonclient.Interface
	targetEnvironments []string
	hash               string
	ownerReference     *metav1.OwnerReference
	waiter             wait.PipelineRunsCompletionWaiter
	pipelineInfo       *model.PipelineInfo
}

func (pipelineCtx *pipelineContext) GetPipelineInfo() *model.PipelineInfo {
	return pipelineCtx.pipelineInfo
}

func (pipelineCtx *pipelineContext) GetHash() string {
	return pipelineCtx.hash
}

func (pipelineCtx *pipelineContext) GetTektonClient() tektonclient.Interface {
	return pipelineCtx.tektonClient
}

func (pipelineCtx *pipelineContext) GetRadixApplication() *radixv1.RadixApplication {
	return pipelineCtx.GetPipelineInfo().GetRadixApplication()
}

func (pipelineCtx *pipelineContext) GetPipelineRunsWaiter() wait.PipelineRunsCompletionWaiter {
	return pipelineCtx.waiter
}

func (pipelineCtx *pipelineContext) GetEnvVars(envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	pipelineCtx.setPipelineRunParamsFromBuild(envVarsMap)
	pipelineCtx.setPipelineRunParamsFromEnvironmentBuilds(envName, envVarsMap)
	return envVarsMap
}

func (pipelineCtx *pipelineContext) SetPipelineTargetEnvironments(environments []string) {
	pipelineCtx.targetEnvironments = environments
}

func (pipelineCtx *pipelineContext) setPipelineRunParamsFromBuild(envVarsMap radixv1.EnvVarsMap) {
	ra := pipelineCtx.GetPipelineInfo().GetRadixApplication()
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

func (pipelineCtx *pipelineContext) setPipelineRunParamsFromEnvironmentBuilds(targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range pipelineCtx.GetPipelineInfo().GetRadixApplication().Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

type NewPipelineContextOption func(pipelineCtx *pipelineContext)

// NewPipelineContext Create new NewPipelineContext instance
func NewPipelineContext(tektonClient tektonclient.Interface, pipelineInfo *model.PipelineInfo, options ...NewPipelineContextOption) Context {
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	pipelineCtx := &pipelineContext{
		tektonClient:   tektonClient,
		pipelineInfo:   pipelineInfo,
		hash:           strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName)),
		ownerReference: ownerReference,
		waiter:         wait.NewPipelineRunsCompletionWaiter(tektonClient),
	}

	for _, option := range options {
		option(pipelineCtx)
	}

	return pipelineCtx
}

// WithPipelineRunsWaiter Set pipeline runs waiter
func WithPipelineRunsWaiter(waiter wait.PipelineRunsCompletionWaiter) NewPipelineContextOption {
	return func(pipelineCtx *pipelineContext) {
		pipelineCtx.waiter = waiter
	}
}
