package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils/conditions"

	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
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

	// Holds information about components to be built
	BuildComponentImages pipeline.BuildComponentImages

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
	CommitID        string
	ImageTag        string
	UseCache        bool
	PushImage       bool
	DeploymentName  string
	FromEnvironment string
	ToEnvironment   string

	RadixConfigFile string
	// Security context
	PodSecurityContext corev1.PodSecurityContext
	// Security context for image builder pods
	BuildKitPodSecurityContext corev1.PodSecurityContext

	ContainerSecurityContext corev1.SecurityContext
	// Images used for copying radix config/building
	TektonPipeline string
	// ImageBuilder Points to the image builder
	ImageBuilder string
	// BuildKitImageBuilder Points to the BuildKit compliant image builder
	BuildKitImageBuilder string
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
}

// InitPipeline Initialize pipeline with step implementations
func InitPipeline(pipelineType *pipeline.Definition,
	pipelineArguments *PipelineArguments,
	stepImplementations ...Step) (*PipelineInfo, error) {

	timestamp := time.Now().Format("20060102150405")
	hash := strings.ToLower(utils.RandStringStrSeed(5, pipelineArguments.JobName))
	radixConfigMapName := fmt.Sprintf("radix-config-2-map-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)
	gitConfigFileName := fmt.Sprintf("radix-git-information-%s-%s-%s", timestamp, pipelineArguments.ImageTag, hash)

	podSecContext := securitycontext.Pod(securitycontext.WithPodFSGroup(defaults.SecurityContextFsGroup),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault))
	buildPodSecContext := securitycontext.Pod(
		securitycontext.WithPodFSGroup(defaults.SecurityContextFsGroup),
		securitycontext.WithPodSeccompProfile(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithPodRunAsNonRoot(conditions.BoolPtr(false)))
	containerSecContext := securitycontext.Container(securitycontext.WithContainerDropAllCapabilities(),
		securitycontext.WithContainerSeccompProfileType(corev1.SeccompProfileTypeRuntimeDefault),
		securitycontext.WithContainerRunAsGroup(defaults.SecurityContextRunAsGroup),
		securitycontext.WithContainerRunAsUser(defaults.SecurityContextRunAsUser))

	pipelineArguments.ContainerSecurityContext = *containerSecContext
	pipelineArguments.PodSecurityContext = *podSecContext
	pipelineArguments.BuildKitPodSecurityContext = *buildPodSecContext

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
func (info *PipelineInfo) SetApplicationConfig(ra *radixv1.RadixApplication) {
	info.RadixApplication = ra

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
