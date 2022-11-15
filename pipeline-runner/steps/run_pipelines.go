package steps

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	pipelineUtils "github.com/equinor/radix-operator/pipeline-runner/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunPipelinesStepImplementation Step to run Tekton pipelines
type RunPipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewRunPipelinesStep Constructor
func NewRunPipelinesStep() model.Step {
	return &RunPipelinesStepImplementation{
		stepType: pipeline.RunPipelinesStep,
	}
}

// ImplementationForType Override of default step method
func (cli *RunPipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *RunPipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: run pipelines step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *RunPipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed run pipelines for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *RunPipelinesStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.PrepareBuildContext != nil && len(pipelineInfo.PrepareBuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Infof("There is no configured sub-pipelines. Skip the step.")
		return nil
	}
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Run pipelines app %s for branch %s and commit %s", appName, branch, commitID)

	job := cli.getRunTektonPipelinesJobConfig(pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Infof("Apply job (%s) to run Tekton pipeline %s", job.Name, appName)
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.GetKubeutil().WaitForCompletionOf(job)
}

func (cli *RunPipelinesStepImplementation) getRunTektonPipelinesJobConfig(pipelineInfo *model.
	PipelineInfo) *batchv1.Job {
	appName := cli.GetAppName()
	action := pipelineDefaults.RadixPipelineActionRun
	envVars := []corev1.EnvVar{
		{
			Name:  defaults.RadixPipelineActionEnvironmentVariable,
			Value: action,
		},
		{
			Name:  defaults.RadixBranchEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.Branch,
		},
		{
			Name:  defaults.RadixAppEnvironmentVariable,
			Value: appName,
		},
		{
			Name:  defaults.RadixPipelineJobEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.JobName,
		},
		{
			Name:  defaults.RadixConfigConfigMapEnvironmentVariable,
			Value: pipelineInfo.RadixConfigMapName,
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
			Name:  defaults.RadixCommitHashEnvironmentVariable,
			Value: pipelineInfo.GitCommitHash,
		},
		{
			Name:  defaults.LogLevel,
			Value: cli.GetEnv().GetLogLevel(),
		},
	}
	return pipelineUtils.CreateActionPipelineJob(defaults.RadixPipelineJobRunPipelinesContainerName, action, pipelineInfo, appName, nil, &envVars)
}
