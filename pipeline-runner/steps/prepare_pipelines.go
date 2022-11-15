package steps

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	pipelineUtils "github.com/equinor/radix-operator/pipeline-runner/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreparePipelinesStepImplementation Step to prepare radixconfig and Tektone pipelines
type PreparePipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPreparePipelinesStep Constructor
func NewPreparePipelinesStep() model.Step {
	return &PreparePipelinesStepImplementation{
		stepType: pipeline.PreparePipelinesStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PreparePipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *PreparePipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: prepare pipelines step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PreparePipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed prepare pipelines for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PreparePipelinesStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Prepare pipelines app %s for branch %s and commit %s", appName, branch, commitID)

	if v1.RadixPipelineType(pipelineInfo.PipelineArguments.PipelineType) == v1.Promote {
		sourceDeploymentGitCommitHash, sourceDeploymentGitBranch, err := cli.getSourceDeploymentGitInfo(appName, pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.DeploymentName)
		if err != nil {
			return err
		}
		pipelineInfo.SourceDeploymentGitCommitHash = sourceDeploymentGitCommitHash
		pipelineInfo.SourceDeploymentGitBranch = sourceDeploymentGitBranch
	}
	job := cli.getPreparePipelinesJobConfig(pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to copy radixconfig to configmap for app %s and prepare Tekton pipeline", job.Name, appName)
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *PreparePipelinesStepImplementation) getPreparePipelinesJobConfig(pipelineInfo *model.PipelineInfo) *batchv1.Job {
	appName := cli.GetAppName()

	action := pipelineDefaults.RadixPipelineActionPrepare
	envVars := []corev1.EnvVar{
		{
			Name:  defaults.RadixPipelineActionEnvironmentVariable,
			Value: action,
		},
		{
			Name:  defaults.RadixAppEnvironmentVariable,
			Value: appName,
		},
		{
			Name:  defaults.RadixConfigConfigMapEnvironmentVariable,
			Value: pipelineInfo.RadixConfigMapName,
		},
		{
			Name:  defaults.RadixGitConfigMapEnvironmentVariable,
			Value: pipelineInfo.GitConfigMapName,
		},
		{
			Name:  defaults.RadixPipelineJobEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.JobName,
		},
		{
			Name:  defaults.RadixConfigFileEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.RadixConfigFile,
		},
		{
			Name:  defaults.RadixBranchEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.Branch,
		},
		{
			Name:  defaults.RadixPipelineTypeEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.PipelineType,
		},
		{
			Name:  defaults.RadixImageTagEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.ImageTag,
		},
		{
			Name:  defaults.RadixPromoteDeploymentEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.DeploymentName,
		},
		{
			Name:  defaults.RadixPromoteFromEnvironmentEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.FromEnvironment,
		},
		{
			Name:  defaults.RadixPromoteToEnvironmentEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.ToEnvironment,
		},
		{
			Name:  defaults.RadixPromoteSourceDeploymentCommitHashEnvironmentVariable,
			Value: pipelineInfo.SourceDeploymentGitCommitHash,
		},
		{
			Name:  defaults.RadixPromoteSourceDeploymentBranchEnvironmentVariable,
			Value: pipelineInfo.SourceDeploymentGitBranch,
		},
		{
			Name:  defaults.LogLevel,
			Value: cli.GetEnv().GetLogLevel(),
		},
		{
			Name:  defaults.RadixGithubWebhookCommitId,
			Value: pipelineInfo.PipelineArguments.CommitID,
		},
	}

	initContainers := cli.getInitContainerCloningRepo(pipelineInfo)

	return pipelineUtils.CreateActionPipelineJob(defaults.RadixPipelineJobPreparePipelinesContainerName, action, pipelineInfo, appName, initContainers, &envVars)

}

func (cli *PreparePipelinesStepImplementation) getSourceDeploymentGitInfo(appName, sourceEnvName, sourceDeploymentName string) (string, string, error) {
	ns := utils.GetEnvironmentNamespace(appName, sourceEnvName)
	rd, err := cli.GetKubeutil().GetRadixDeployment(ns, sourceDeploymentName)
	if err != nil {
		return "", "", err
	}
	gitHash := getGitCommitHashFromDeployment(rd)
	gitBranch := rd.Annotations[kube.RadixBranchAnnotation]
	return gitHash, gitBranch, err
}

func (cli *PreparePipelinesStepImplementation) getInitContainerCloningRepo(pipelineInfo *model.PipelineInfo) []corev1.Container {
	registration := cli.GetRegistration()
	configBranch := applicationconfig.GetConfigBranch(registration)
	sshURL := registration.Spec.CloneURL
	return git.CloneInitContainersWithContainerName(sshURL, configBranch, git.CloneConfigContainerName,
		pipelineInfo.PipelineArguments.ContainerSecurityContext)
}
