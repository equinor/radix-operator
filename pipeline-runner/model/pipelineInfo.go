package model

import (
	"fmt"
	"path/filepath"

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
	RadixRegistration *radixv1.RadixRegistration
	RadixApplication  *radixv1.RadixApplication
	BuildSecret       *corev1.Secret
	PipelineArguments PipelineArguments
	Steps             []Step

	// Temporary data
	TargetEnvironments []string

	// GitCommitHash is derived by inspecting HEAD commit after cloning user repository in prepare-pipelines step.
	// not to be confused with PipelineInfo.PipelineArguments.CommitID
	GitCommitHash string
	GitTags       string

	// Holds information about components to be built for each environment
	BuildComponentImages pipeline.EnvironmentBuildComponentImages

	// Hold information about components to be deployed
	DeployEnvironmentComponentImages pipeline.DeployEnvironmentComponentImages

	// Pipeline job build context
	BuildContext        *BuildContext
	StopPipeline        bool
	StopPipelineMessage string

	// Promotion job git info
	SourceDeploymentGitCommitHash string
	SourceDeploymentGitBranch     string
}

// Builder Holds info about the builder arguments
type Builder struct {
	ResourcesLimitsMemory   string
	ResourcesLimitsCPU      string
	ResourcesRequestsCPU    string
	ResourcesRequestsMemory string
}

type ApplyConfigOptions struct {
	DeployExternalDNS bool
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
	RefreshBuildCache     *bool
	PushImage             bool
	DeploymentName        string
	FromEnvironment       string
	ToEnvironment         string
	ComponentsToDeploy    []string
	TriggeredFromWebhook  bool
	RadixConfigFile       string

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
	// ApplyConfigOptions holds options for applying radixconfig
	ApplyConfigOptions ApplyConfigOptions
	// GitWorkspace is the path to the git workspace
	GitWorkspace string
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition, pipelineArguments *PipelineArguments, stepImplementations ...Step) (*PipelineInfo, error) {
	stepImplementationsForType, err := getStepStepImplementationsFromType(pipelineType, stepImplementations...)
	if err != nil {
		return nil, err
	}

	return &PipelineInfo{
		Definition:        pipelineType,
		PipelineArguments: *pipelineArguments,
		Steps:             stepImplementationsForType,
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
func (p *PipelineInfo) SetApplicationConfig(applicationConfig *application.ApplicationConfig) {
	p.RadixApplication = applicationConfig.GetRadixApplicationConfig()
}

// SetGitAttributes Set git attributes to be used later by other steps
func (p *PipelineInfo) SetGitAttributes(gitCommitHash, gitTags string) {
	p.GitCommitHash = gitCommitHash
	p.GitTags = gitTags
}

// IsPipelineType Check pipeline type
func (p *PipelineInfo) IsPipelineType(pipelineType radixv1.RadixPipelineType) bool {
	return p.GetRadixPipelineType() == pipelineType
}

func (p *PipelineInfo) IsUsingBuildKit() bool {
	return p.RadixApplication.Spec.Build != nil && p.RadixApplication.Spec.Build.UseBuildKit != nil && *p.RadixApplication.Spec.Build.UseBuildKit
}

func (p *PipelineInfo) IsUsingBuildCache() bool {
	if !p.IsUsingBuildKit() {
		return false
	}

	useBuildCache := p.RadixApplication.Spec.Build == nil || p.RadixApplication.Spec.Build.UseBuildCache == nil || *p.RadixApplication.Spec.Build.UseBuildCache
	if p.PipelineArguments.OverrideUseBuildCache != nil {
		useBuildCache = *p.PipelineArguments.OverrideUseBuildCache
	}

	return useBuildCache
}

func (p *PipelineInfo) IsRefreshingBuildCache() bool {
	if !p.IsUsingBuildCache() {
		return false
	}
	if p.PipelineArguments.RefreshBuildCache != nil {
		return *p.PipelineArguments.RefreshBuildCache
	}
	return false
}

// GetRadixConfigBranch Get config branch
func (p *PipelineInfo) GetRadixConfigBranch() string {
	return p.RadixRegistration.Spec.ConfigBranch
}

// GetRadixConfigFileInWorkspace Get radix config file
func (p *PipelineInfo) GetRadixConfigFileInWorkspace() string {
	return filepath.Join(p.PipelineArguments.GitWorkspace, p.PipelineArguments.RadixConfigFile)
}

// GetGitWorkspace Get git workspace
func (p *PipelineInfo) GetGitWorkspace() string {
	return p.PipelineArguments.GitWorkspace
}

// GetAppName Get app name
func (p *PipelineInfo) GetAppName() string {
	return p.PipelineArguments.AppName
}

// GetRadixPipelineType Get radix pipeline type
func (p *PipelineInfo) GetRadixPipelineType() radixv1.RadixPipelineType {
	return radixv1.RadixPipelineType(p.PipelineArguments.PipelineType)
}

// GetRadixApplication Get radix application
func (p *PipelineInfo) GetRadixApplication() *radixv1.RadixApplication {
	return p.RadixApplication
}

// GetBranch Get branch
func (p *PipelineInfo) GetBranch() string {
	return p.PipelineArguments.Branch
}

// GetAppNamespace Get app namespace
func (p *PipelineInfo) GetAppNamespace() string {
	return utils.GetAppNamespace(p.PipelineArguments.AppName)
}

// GetRadixImageTag Get radix image tag
func (p *PipelineInfo) GetRadixImageTag() string {
	return p.PipelineArguments.ImageTag
}

// GetRadixPipelineJobName Get radix pipeline job name
func (p *PipelineInfo) GetRadixPipelineJobName() string {
	return p.PipelineArguments.JobName
}

// GetDNSConfig Get DNS config
func (p *PipelineInfo) GetDNSConfig() *dnsaliasconfig.DNSConfig {
	return p.PipelineArguments.DNSConfig
}

// GetRadixDeployToEnvironment Get radix deploy to environment
func (p *PipelineInfo) GetRadixDeployToEnvironment() string {
	return p.PipelineArguments.ToEnvironment
}

// GetRadixPromoteDeployment Get radix promote deployment
func (p *PipelineInfo) GetRadixPromoteDeployment() string {
	return p.PipelineArguments.DeploymentName
}

// GetRadixPromoteFromEnvironment Get radix promote from environment
func (p *PipelineInfo) GetRadixPromoteFromEnvironment() string {
	return p.PipelineArguments.FromEnvironment
}

// SetBuildContext Set build context
func (p *PipelineInfo) SetBuildContext(context *BuildContext) *PipelineInfo {
	p.BuildContext = context
	return p
}
