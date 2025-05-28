package model

import (
	"fmt"
	"path/filepath"

	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/hash"
	corev1 "k8s.io/api/core/v1"
)

// PipelineInfo Holds info about the pipeline to run
type PipelineInfo struct {
	Definition        *pipeline.Definition
	RadixApplication  *radixv1.RadixApplication
	BuildSecret       *corev1.Secret
	PipelineArguments PipelineArguments
	Steps             []Step

	// TargetEnvironments holds information about which environments to build and deploy.
	// It is populated by the prepare-pipeline step by inspecting PipelineArguments
	TargetEnvironments []TargetEnvironment

	// GitCommitHash is derived by inspecting HEAD commit after cloning user repository in prepare-pipelines step.
	// not to be confused with PipelineInfo.PipelineArguments.CommitID
	GitCommitHash string
	GitTags       string

	// Holds information about components to be built for each environment
	BuildComponentImages pipeline.EnvironmentBuildComponentImages

	// Hold information about components to be deployed
	DeployEnvironmentComponentImages pipeline.DeployEnvironmentComponentImages

	// Pipeline job build context
	BuildContext *BuildContext
	// EnvironmentSubPipelinesToRun Sub-pipeline pipeline file named, if they are configured to be run
	EnvironmentSubPipelinesToRun []EnvironmentSubPipelineToRun
	StopPipeline                 bool
	StopPipelineMessage          string
}

type TargetEnvironment struct {
	Environment           string
	ActiveRadixDeployment *radixv1.RadixDeployment
}

// CompareApplicationWithDeploymentHash generates a hash for ra and compares it
// with the values stored in annotation radix.equinor.com/radix-config-hash of the ActiveRadixDeployment.
// Returns true if the two hashes match, and false if they do not match, or ActiveRadixDeployment is nil or the annotation does not exist or has an empty value
func (t TargetEnvironment) CompareApplicationWithDeploymentHash(ra *radixv1.RadixApplication) (bool, error) {
	if t.ActiveRadixDeployment == nil {
		return false, nil
	}
	currentRdConfigHash := t.ActiveRadixDeployment.GetAnnotations()[kube.RadixConfigHash]
	if len(currentRdConfigHash) == 0 {
		return false, nil
	}
	return hash.CompareRadixApplicationHash(currentRdConfigHash, ra)
}

// EnvironmentSubPipelineToRun An application environment sub-pipeline to be run
type EnvironmentSubPipelineToRun struct {
	// Environment name
	Environment string
	// PipelineFile Name of a sub-pipeline file, which need to be run
	PipelineFile string
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
	// Deprecated: use GitRef instead
	Branch string
	// GitRef Branch or tag to build from
	//
	// required: false
	// example: master
	GitRef string `json:"gitRef,omitempty"`
	// GitRefType When the pipeline job should be built from branch or tag specified in GitRef:
	// - branch
	// - tag
	// - <empty> - either branch or tag
	//
	// required false
	// enum: branch,tag,""
	// example: "branch"
	GitRefType string `json:"gitRefType,omitempty"`
	// CommitID is sent from GitHub webhook. not to be confused with PipelineInfo.GitCommitHash
	CommitID string
	ImageTag string
	// OverrideUseBuildCache override default or configured build cache option
	OverrideUseBuildCache *bool
	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	RefreshBuildCache    *bool
	PushImage            bool
	DeploymentName       string
	FromEnvironment      string
	ToEnvironment        string
	ComponentsToDeploy   []string
	TriggeredFromWebhook bool
	RadixConfigFile      string

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

// SetGitAttributes Set git attributes to be used later by other steps
func (p *PipelineInfo) SetGitAttributes(gitCommitHash, gitTags string) {
	p.GitCommitHash = gitCommitHash
	p.GitTags = gitTags
}

// IsPipelineType Check pipeline type
func (p *PipelineInfo) IsPipelineType(pipelineType radixv1.RadixPipelineType) bool {
	return p.GetRadixPipelineType() == pipelineType
}

// IsUsingBuildKit Check if buildkit should be used
func (p *PipelineInfo) IsUsingBuildKit() bool {
	return p.RadixApplication != nil && p.RadixApplication.Spec.Build != nil &&
		p.RadixApplication.Spec.Build.UseBuildKit != nil && *p.RadixApplication.Spec.Build.UseBuildKit
}

// IsUsingBuildCache Check if build cache should be used
func (p *PipelineInfo) IsUsingBuildCache() bool {
	if !p.IsUsingBuildKit() {
		return false
	}
	if p.PipelineArguments.OverrideUseBuildCache != nil {
		return *p.PipelineArguments.OverrideUseBuildCache
	}
	return p.RadixApplication.Spec.Build != nil && (p.RadixApplication.Spec.Build.UseBuildCache == nil || *p.RadixApplication.Spec.Build.UseBuildCache)
}

// IsRefreshingBuildCache Check if build cache should be refreshed
func (p *PipelineInfo) IsRefreshingBuildCache() bool {
	if !p.IsUsingBuildKit() || p.PipelineArguments.RefreshBuildCache == nil {
		return false
	}
	return *p.PipelineArguments.RefreshBuildCache
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

// GetGitRef Get branch or tag
func (p *PipelineInfo) GetGitRef() string {
	return p.PipelineArguments.GetGitRefOrDefault()
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

// GetGitRefType Get git event ref type
func (p *PipelineInfo) GetGitRefType() string {
	return p.PipelineArguments.GitRefType
}

// GetGitRefTypeOrDefault Get git event ref type or "branch" by default
func (p *PipelineInfo) GetGitRefTypeOrDefault() string {
	if p.PipelineArguments.GitRefType == "" {
		return string(radixv1.GitRefBranch)
	}
	return p.PipelineArguments.GitRefType
}

// GetGitRefOrDefault Get git event ref or "branch" by default
func (args *PipelineArguments) GetGitRefOrDefault() string {
	if args.GitRef == "" {
		return args.Branch
	}
	return args.GitRef
}

// GetGitRefOrDefault Get git event ref or "branch" by default
func (p *PipelineInfo) GetGitRefOrDefault() string {
	return p.PipelineArguments.GetGitRefOrDefault()
}
