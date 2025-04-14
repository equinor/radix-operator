package preparepipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/equinor/radix-common/utils"
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

// Context of the pipeline
type Context interface {
	// LoadRadixAppConfig Load Radix config file and create RadixApplication
	LoadRadixAppConfig() (*radixv1.RadixApplication, error)
	// GetBuildContext Get build context
	GetBuildContext() (*model.BuildContext, error)
	// GetEnvironmentSubPipelinesToRun Get environment sub-pipelines to run
	GetEnvironmentSubPipelinesToRun() ([]model.EnvironmentSubPipelineToRun, error)
	// GetPipelineInfo Get pipeline info
	GetPipelineInfo() *model.PipelineInfo
	// SetPipelineTargetEnvironments Set target environments for the pipeline job
	SetPipelineTargetEnvironments(environments []string)
}

type pipelineContext struct {
	radixClient        radixclient.Interface
	kubeClient         kubernetes.Interface
	tektonClient       tektonclient.Interface
	targetEnvironments []string
	hash               string
	ownerReference     *metav1.OwnerReference
	pipelineInfo       *model.PipelineInfo
}

func (pipelineCtx *pipelineContext) GetPipelineInfo() *model.PipelineInfo {
	return pipelineCtx.pipelineInfo
}

func (pipelineCtx *pipelineContext) HasCommitID() bool {
	return len(pipelineCtx.pipelineInfo.PipelineArguments.CommitID) > 0
}

// GetHash Hash, common for all pipeline Kubernetes object names
func (pipelineCtx *pipelineContext) GetHash() string {
	return pipelineCtx.hash
}

// GetKubeClient Kubernetes client
func (pipelineCtx *pipelineContext) GetKubeClient() kubernetes.Interface {
	return pipelineCtx.kubeClient
}

// GetTektonClient Tekton client
func (pipelineCtx *pipelineContext) GetTektonClient() tektonclient.Interface {
	return pipelineCtx.tektonClient
}

// GetRadixApplication Gets the RadixApplication, loaded from the config-map
func (pipelineCtx *pipelineContext) GetRadixApplication() *radixv1.RadixApplication {
	return pipelineCtx.GetPipelineInfo().GetRadixApplication()
}

// GetEnvVars Gets build env vars
func (pipelineCtx *pipelineContext) GetEnvVars(envName string) radixv1.EnvVarsMap {
	envVarsMap := make(radixv1.EnvVarsMap)
	pipelineCtx.setPipelineRunParamsFromBuild(envVarsMap)
	pipelineCtx.setPipelineRunParamsFromEnvironmentBuilds(envName, envVarsMap)
	return envVarsMap
}

func (pipelineCtx *pipelineContext) SetPipelineTargetEnvironments(environments []string) {
	pipelineCtx.targetEnvironments = environments
}

// GetPipelineTargetEnvironments Get target environments for the pipeline job
func (pipelineCtx *pipelineContext) GetPipelineTargetEnvironments() []string {
	return pipelineCtx.targetEnvironments
}

func (pipelineCtx *pipelineContext) GetRadixClient() radixclient.Interface {
	return pipelineCtx.radixClient
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

func (pipelineCtx *pipelineContext) getGitHash() (string, error) {
	// getGitHash return git commit to which the user repository should be reset before parsing sub-pipelines.
	pipelineArgs := pipelineCtx.pipelineInfo.PipelineArguments
	pipelineType := pipelineCtx.pipelineInfo.GetRadixPipelineType()
	if pipelineType == radixv1.ApplyConfig {
		return "", nil
	}

	if pipelineType == radixv1.Promote {
		sourceRdHashFromAnnotation := pipelineCtx.pipelineInfo.SourceDeploymentGitCommitHash
		sourceDeploymentGitBranch := pipelineCtx.pipelineInfo.SourceDeploymentGitBranch
		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}
		if sourceDeploymentGitBranch == "" {
			log.Info().Msg("source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := git.GetCommitHashFromHead(pipelineCtx.GetPipelineInfo().GetGitWorkspace(), sourceDeploymentGitBranch)
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if pipelineType == radixv1.Deploy {
		pipelineJobBranch := ""
		re := applicationconfig.GetEnvironmentFromRadixApplication(pipelineCtx.GetPipelineInfo().GetRadixApplication(), pipelineArgs.ToEnvironment)
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
		gitHash, err := git.GetCommitHashFromHead(pipelineArgs.GitWorkspace, pipelineJobBranch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if pipelineType == radixv1.BuildDeploy || pipelineType == radixv1.Build {
		gitHash, err := git.GetCommitHash(pipelineArgs.GitWorkspace, pipelineArgs.CommitID, pipelineArgs.Branch)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}
	return "", fmt.Errorf("unknown pipeline type %s", pipelineType)
}

type NewPipelineContextOption func(pipelineCtx *pipelineContext)

// NewPipelineContext Create new NewPipelineContext instance
func NewPipelineContext(kubeClient kubernetes.Interface, radixClient radixclient.Interface, tektonClient tektonclient.Interface, pipelineInfo *model.PipelineInfo, options ...NewPipelineContextOption) Context {
	ownerReference := ownerreferences.GetOwnerReferenceOfJobFromLabels()
	pipelineCtx := &pipelineContext{
		kubeClient:     kubeClient,
		radixClient:    radixClient,
		tektonClient:   tektonClient,
		pipelineInfo:   pipelineInfo,
		hash:           strings.ToLower(utils.RandStringStrSeed(5, pipelineInfo.PipelineArguments.JobName)),
		ownerReference: ownerReference,
	}

	for _, option := range options {
		option(pipelineCtx)
	}

	return pipelineCtx
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
