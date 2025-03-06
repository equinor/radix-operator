package pipeline

import (
	"fmt"
	"github.com/equinor/radix-tekton/pkg/models/env"
	"regexp"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	ownerreferences "github.com/equinor/radix-operator/pipeline-runner/utils/owner_references"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type pipelineContext struct {
	radixClient        radixclient.Interface
	kubeClient         kubernetes.Interface
	tektonClient       tektonclient.Interface
	env                env.Env
	radixApplication   *v1.RadixApplication
	targetEnvironments map[string]bool
	hash               string
	ownerReference     *metav1.OwnerReference
	waiter             wait.PipelineRunsCompletionWaiter
	pipelineInfo       *model.PipelineInfo
}

func (ctx *pipelineContext) GetPipelineInfo() *model.PipelineInfo {
	return ctx.pipelineInfo
}

func (ctx *pipelineContext) GetHash() string {
	return ctx.hash
}

func (ctx *pipelineContext) GetKubeClient() kubernetes.Interface {
	return ctx.kubeClient
}

func (ctx *pipelineContext) GetTektonClient() tektonclient.Interface {
	return ctx.tektonClient
}

func (ctx *pipelineContext) GetRadixApplication() *v1.RadixApplication {
	return ctx.radixApplication
}

func (ctx *pipelineContext) GetPipelineRunsWaiter() wait.PipelineRunsCompletionWaiter {
	return ctx.waiter
}

func (ctx *pipelineContext) GetEnvVars(envName string) v1.EnvVarsMap {
	envVarsMap := make(v1.EnvVarsMap)
	ctx.setPipelineRunParamsFromBuild(envVarsMap)
	ctx.setPipelineRunParamsFromEnvironmentBuilds(envName, envVarsMap)
	return envVarsMap
}

func (ctx *pipelineContext) setPipelineRunParamsFromBuild(envVarsMap v1.EnvVarsMap) {
	if ctx.radixApplication.Spec.Build == nil {
		return
	}
	setBuildIdentity(envVarsMap, ctx.radixApplication.Spec.Build.SubPipeline)
	setBuildVariables(envVarsMap, ctx.radixApplication.Spec.Build.SubPipeline, ctx.radixApplication.Spec.Build.Variables)
}

func setBuildVariables(envVarsMap v1.EnvVarsMap, subPipeline *v1.SubPipeline, variables v1.EnvVarsMap) {
	if subPipeline != nil {
		setVariablesToEnvVarsMap(envVarsMap, subPipeline.Variables) // sub-pipeline variables have higher priority over build variables
		return
	}
	setVariablesToEnvVarsMap(envVarsMap, variables) // keep for backward compatibility
}

func setVariablesToEnvVarsMap(envVarsMap v1.EnvVarsMap, variables v1.EnvVarsMap) {
	for name, envVar := range variables {
		envVarsMap[name] = envVar
	}
}

func setBuildIdentity(envVarsMap v1.EnvVarsMap, subPipeline *v1.SubPipeline) {
	if subPipeline != nil {
		setIdentityToEnvVarsMap(envVarsMap, subPipeline.Identity)
	}
}

func setIdentityToEnvVarsMap(envVarsMap v1.EnvVarsMap, identity *v1.Identity) {
	if identity == nil || identity.Azure == nil {
		return
	}
	if len(identity.Azure.ClientId) > 0 {
		envVarsMap[defaults.AzureClientIdEnvironmentVariable] = identity.Azure.ClientId // if build env-var or build environment env-var have this variable explicitly set, it will override this identity set env-var
	} else {
		delete(envVarsMap, defaults.AzureClientIdEnvironmentVariable)
	}
}

func (ctx *pipelineContext) setPipelineRunParamsFromEnvironmentBuilds(targetEnv string, envVarsMap v1.EnvVarsMap) {
	for _, buildEnv := range ctx.radixApplication.Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

// getGitHash return git commit to which the user repository should be reset before parsing sub-pipelines.
func (ctx *pipelineContext) getGitHash() (string, error) {
	if ctx.env.GetRadixPipelineType() == v1.Build || ctx.env.GetRadixPipelineType() == v1.ApplyConfig {
		log.Info().Msg("Skipping sub-pipelines.")
		return "", nil
	}

	if ctx.env.GetRadixPipelineType() == v1.Promote {
		sourceRdHashFromAnnotation := ctx.env.GetSourceDeploymentGitCommitHash()
		sourceDeploymentGitBranch := ctx.env.GetSourceDeploymentGitBranch()
		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}
		if sourceDeploymentGitBranch == "" {
			log.Info().Msg("source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := git.GetGitCommitHashFromHead(ctx.env.GetGitRepositoryWorkspace(), sourceDeploymentGitBranch)
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if ctx.env.GetRadixPipelineType() == v1.Deploy {
		pipelineJobBranch := ""
		re := applicationconfig.GetEnvironmentFromRadixApplication(ctx.radixApplication, ctx.env.GetRadixDeployToEnvironment())
		if re != nil {
			pipelineJobBranch = re.Build.From
		}
		if pipelineJobBranch == "" {
			log.Info().Msg("deploy job with no build branch, skipping sub-pipelines.")
			return "", nil
		}
		if containsRegex(pipelineJobBranch) {
			log.Info().Msg("deploy job with build branch having regex pattern, skipping sub-pipelines.")
			return "", nil
		}
		gitHash, err := git.GetGitCommitHashFromHead(ctx.env.GetGitRepositoryWorkspace(), pipelineJobBranch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if ctx.env.GetRadixPipelineType() == v1.BuildDeploy {
		gitHash, err := git.GetGitCommitHash(ctx.env.GetGitRepositoryWorkspace(), ctx.env)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}
	return "", fmt.Errorf("unknown pipeline type %s", ctx.env.GetRadixPipelineType())
}

type NewPipelineContextOption func(ctx *pipelineContext)

// NewPipelineContext Create new NewPipelineContext instance
func NewPipelineContext(kubeClient kubernetes.Interface, radixClient radixclient.Interface, tektonClient tektonclient.Interface, pipelineInfo *model.PipelineInfo, options ...NewPipelineContextOption) model.Context {
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	ctx := &pipelineContext{
		kubeClient:     kubeClient,
		radixClient:    radixClient,
		tektonClient:   tektonClient,
		pipelineInfo:   pipelineInfo,
		hash:           strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName)),
		ownerReference: ownerReference,
		waiter:         wait.NewPipelineRunsCompletionWaiter(tektonClient),
	}

	for _, option := range options {
		option(ctx)
	}

	return ctx
}

func WithPipelineRunsWaiter(waiter wait.PipelineRunsCompletionWaiter) NewPipelineContextOption {
	return func(ctx *pipelineContext) {
		ctx.waiter = waiter
	}
}

func containsRegex(value string) bool {
	if simpleSentence := regexp.MustCompile(`^[a-zA-Z0-9\s\.\-/]+$`); simpleSentence.MatchString(value) {
		return false
	}
	// Regex value that looks for typical regex special characters
	if specialRegexChars := regexp.MustCompile(`[\[\](){}.*+?^$\\|]`); specialRegexChars.FindStringIndex(value) != nil {
		_, err := regexp.Compile(value)
		return err == nil
	}
	return false
}
