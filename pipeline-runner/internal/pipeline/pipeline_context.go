package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	ownerreferences "github.com/equinor/radix-operator/pipeline-runner/utils/owner_references"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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

func (ctx *pipelineContext) GetRadixApplication() *radixv1.RadixApplication {
	return ctx.GetPipelineInfo().GetRadixApplication()
}

func (ctx *pipelineContext) GetPipelineRunsWaiter() wait.PipelineRunsCompletionWaiter {
	return ctx.waiter
}

func (ctx *pipelineContext) GetEnvVars(envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	ctx.setPipelineRunParamsFromBuild(envVarsMap)
	ctx.setPipelineRunParamsFromEnvironmentBuilds(envName, envVarsMap)
	return envVarsMap
}

func (ctx *pipelineContext) setPipelineRunParamsFromBuild(envVarsMap radixv1.EnvVarsMap) {
	ra := ctx.GetPipelineInfo().GetRadixApplication()
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

func (ctx *pipelineContext) setPipelineRunParamsFromEnvironmentBuilds(targetEnv string, envVarsMap radixv1.EnvVarsMap) {
	for _, buildEnv := range ctx.GetPipelineInfo().GetRadixApplication().Spec.Environments {
		if strings.EqualFold(buildEnv.Name, targetEnv) {
			setBuildIdentity(envVarsMap, buildEnv.SubPipeline)
			setBuildVariables(envVarsMap, buildEnv.SubPipeline, buildEnv.Build.Variables)
		}
	}
}

// getGitHash return git commit to which the user repository should be reset before parsing sub-pipelines.
func (ctx *pipelineContext) getGitHash() (string, error) {
	pipelineArgs := ctx.pipelineInfo.PipelineArguments
	pipelineType := ctx.pipelineInfo.GetRadixPipelineType()
	if pipelineType == radixv1.Build || pipelineType == radixv1.ApplyConfig {
		log.Info().Msg("Skipping sub-pipelines.")
		return "", nil
	}

	if pipelineType == radixv1.Promote {
		sourceRdHashFromAnnotation := ctx.pipelineInfo.SourceDeploymentGitCommitHash
		sourceDeploymentGitBranch := ctx.pipelineInfo.SourceDeploymentGitBranch
		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}
		if sourceDeploymentGitBranch == "" {
			log.Info().Msg("source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := git.GetGitCommitHashFromHead(ctx.GetPipelineInfo().GetGitWorkspace(), sourceDeploymentGitBranch)
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if pipelineType == radixv1.Deploy {
		pipelineJobBranch := ""
		re := applicationconfig.GetEnvironmentFromRadixApplication(ctx.GetPipelineInfo().GetRadixApplication(), pipelineArgs.ToEnvironment)
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
		gitHash, err := git.GetGitCommitHashFromHead(pipelineArgs.GitWorkspace, pipelineJobBranch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if pipelineType == radixv1.BuildDeploy {
		gitHash, err := git.GetGitCommitHash(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}
	return "", fmt.Errorf("unknown pipeline type %s", pipelineType)
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
	}

	for _, option := range options {
		option(ctx)
	}

	return ctx
}

// WithPipelineRunsWaiter Set pipeline runs waiter
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
