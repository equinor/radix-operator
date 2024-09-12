package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Definition        *pipeline.Definition
	RadixApplication  *radixv1.RadixApplication
	BuildSecret       *corev1.Secret
	PipelineArguments PipelineArguments
	Steps             []Step

	// Temporary data
	RadixConfigMapName string
	GitConfigMapName   string
	TargetEnvironments []string

	// GitCommitHash is derived by inspecting HEAD commit after cloning user repository in prepare-pipelines step.
	// not to be confused with PipelineInfo.PipelineArguments.CommitID
	GitCommitHash string
	GitTags       string

	// Holds information about components to be built for each environment
	BuildComponentImages pipeline.EnvironmentBuildComponentImages

	// Hold information about components to be deployed
	DeployEnvironmentComponentImages pipeline.DeployEnvironmentComponentImages

	// Prepare pipeline job build context
	PrepareBuildContext *PrepareBuildContext
	StopPipeline        bool
	StopPipelineMessage string

	// Promotion job git info
	SourceDeploymentGitCommitHash string
	SourceDeploymentGitBranch     string
}

// Builder Holds info about the builder arguments
type Builder struct {
	ResourcesLimitsMemory   string
	ResourcesRequestsCPU    string
	ResourcesRequestsMemory string
}

// PipelineArguments Holds arguments for the pipeline
type PipelineArguments struct {
	PipelineType string
	JobName      string
	Branch       string
	// CommitID is sent from GitHub webhook. not to be confused with PipelineInfo.GitCommitHash
	CommitID string
	ImageTag string
	// OverrideUseBuildCache override default or configured build cache option
	OverrideUseBuildCache *bool
	PushImage             bool
	DeploymentName        string
	FromEnvironment       string
	ToEnvironment         string
	ComponentsToDeploy    []string

	RadixConfigFile string

	// Images used for copying radix config/building
	TektonPipeline string
	// ImageBuilder Points to the image builder (repository and tag only)
	ImageBuilder string
	// BuildKitImageBuilder Points to the BuildKit compliant image builder (repository and tag only)
	BuildKitImageBuilder string
	// GitCloneNsLookupImage defines image containing nslookup.
	// Used as option to the CloneInitContainers function.
	GitCloneNsLookupImage string
	// GitCloneGitImage defines image containing git cli.
	// Must support running as user 65534.
	// Used as option to the CloneInitContainers function.
	GitCloneGitImage string
	// GitCloneBashImage defines image with bash.
	// Used as option to the CloneInitContainers function.
	GitCloneBashImage string
	// SeccompProfileFileName Filename of the seccomp profile injected by daemonset, relative to the /var/lib/kubelet/seccomp directory on node
	SeccompProfileFileName string
	// Used for tagging meta-information
	Clustertype string
	// RadixZone  The radix zone.
	RadixZone string
	// Clustername The name of the cluster
	Clustername string
	// ContainerRegistry The name of the container registry
	ContainerRegistry string
	// AppContainerRegistry the name of the app container registry
	AppContainerRegistry string
	// SubscriptionId Azure subscription ID
	SubscriptionId string
	// Used to indicate debugging session
	Debug bool
	// Image tag names for components: component-name:image-tag
	ImageTagNames map[string]string
	LogLevel      string
	AppName       string
	Builder       Builder
	DNSConfig     *dnsaliasconfig.DNSConfig

	// Name of secret with .dockerconfigjson key containing docker auths. Optional.
	// Used to authenticate external container registries when using buildkit to build dockerfiles.
	ExternalContainerRegistryDefaultAuthSecret string
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
	pipelineArguments *PipelineArguments,
	stepImplementations ...Step) (*PipelineInfo, error) {

	timestamp := time.Now().Format("20060102150405")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineArguments.JobName))
	radixConfigMapName := fmt.Sprintf("radix-config-2-map-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)
	gitConfigFileName := fmt.Sprintf("radix-git-information-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)

	stepImplementationsForType, err := getStepStepImplementationsFromType(pipelineType, stepImplementations...)
	if err != nil {
		return nil, err
	}

	return &PipelineInfo{
		Definition:         pipelineType,
		PipelineArguments:  *pipelineArguments,
		Steps:              stepImplementationsForType,
		RadixConfigMapName: radixConfigMapName,
		GitConfigMapName:   gitConfigFileName,
	}, nil
}

func getStepStepImplementationsFromType(pipelineType *pipeline.Definition, allStepImplementations ...Step) ([]Step, error) {
	stepImplementations := make([]Step, 0)

	for _, step := range pipelineType.Steps {
		stepImplementation := getStepImplementationForStepType(step, allStepImplementations)
		if stepImplementation == nil {
			return nil, fmt.Errorf("no step implementation found by type %s", stepImplementation)
		}

		stepImplementations = append(stepImplementations, stepImplementation)
	}

	return stepImplementations, nil
}

func getStepImplementationForStepType(stepType pipeline.StepType, allStepImplementations []Step) Step {
	for _, stepImplementation := range allStepImplementations {
		implementsType := stepImplementation.ImplementationForType()

		if stepType == implementsType {
			return stepImplementation
		}
	}

	return nil
}

// SetApplicationConfig Set radixconfig to be used later by other steps, as well
// as deriving info from the config
func (info *PipelineInfo) SetApplicationConfig(applicationConfig *application.ApplicationConfig) {
	info.RadixApplication = applicationConfig.GetRadixApplicationConfig()

	// Obtain metadata for rest of pipeline
	targetEnvironments := application.GetTargetEnvironments(info.PipelineArguments.Branch, info.RadixApplication)

	// For deploy-only pipeline
	if info.IsPipelineType(radixv1.Deploy) &&
		!slice.Any(targetEnvironments, func(s string) bool { return s == info.PipelineArguments.ToEnvironment }) {
		targetEnvironments = append(targetEnvironments, info.PipelineArguments.ToEnvironment)
	}

	info.TargetEnvironments = targetEnvironments
}

// SetGitAttributes Set git attributes to be used later by other steps
func (info *PipelineInfo) SetGitAttributes(gitCommitHash, gitTags string) {
	info.GitCommitHash = gitCommitHash
	info.GitTags = gitTags
}

// IsPipelineType Check pipeline type
func (info *PipelineInfo) IsPipelineType(pipelineType radixv1.RadixPipelineType) bool {
	return info.PipelineArguments.PipelineType == string(pipelineType)
}

func (info *PipelineInfo) IsUsingBuildKit() bool {
	return info.RadixApplication.Spec.Build != nil && info.RadixApplication.Spec.Build.UseBuildKit != nil && *info.RadixApplication.Spec.Build.UseBuildKit
}

func (info *PipelineInfo) IsUsingBuildCache() bool {
	if !info.IsUsingBuildKit() {
		return false
	}

	useBuildCache := info.RadixApplication.Spec.Build == nil || info.RadixApplication.Spec.Build.UseBuildCache == nil || *info.RadixApplication.Spec.Build.UseBuildCache
	if info.PipelineArguments.OverrideUseBuildCache != nil {
		useBuildCache = *info.PipelineArguments.OverrideUseBuildCache
	}

	return useBuildCache
}
