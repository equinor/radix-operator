package steps

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
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

// PrepareTektonPipelineStepImplementation Step to run custom pipeline
type PrepareTektonPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPrepareTektonPipelineStep Constructor
func NewPrepareTektonPipelineStep() model.Step {
	return &PrepareTektonPipelineStepImplementation{
		stepType: pipeline.PrepareTektonPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: prepare tekton pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed prepare tekton pipeline for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PrepareTektonPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Run tekton pipeline app %s for branch %s and commit %s", appName, branch, commitID)

	job := cli.getPrepareTektonPipelinesJobConfig(pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to copy radixconfig to configmap for app %s", job.Name, appName)
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *PrepareTektonPipelineStepImplementation) getPrepareTektonPipelinesJobConfig(pipelineInfo *model.PipelineInfo) *batchv1.Job {
	appName := cli.GetAppName()

	action := "prepare"
	envVars := []corev1.EnvVar{
		{
			Name:  defaults.RadixTektonActionEnvironmentVariable,
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
			Name:  defaults.RadixPipelineRunEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.RadixPipelineRun,
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
			Name:  defaults.LogLevel,
			Value: cli.GetEnv().GetLogLevel(),
		},
	}

	initContainers := cli.getInitContainers(pipelineInfo)

	return pipelineUtils.CreateTektonPipelineJob(action, pipelineInfo, appName, initContainers, &envVars)

}

func (cli *PrepareTektonPipelineStepImplementation) getInitContainers(pipelineInfo *model.PipelineInfo) []corev1.Container {
	registration := cli.GetRegistration()
	configBranch := applicationconfig.GetConfigBranch(registration)
	sshURL := registration.Spec.CloneURL
	return git.CloneInitContainersWithContainerName(sshURL, configBranch, git.CloneConfigContainerName,
		pipelineInfo.PipelineArguments.ContainerSecurityContext)
}
