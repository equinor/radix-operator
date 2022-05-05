package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
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

// RunTektonPipelineStepImplementation Step to run custom pipeline
type RunTektonPipelineStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewTektonPipelineStep Constructor
func NewTektonPipelineStep() model.Step {
	return &RunTektonPipelineStepImplementation{
		stepType: pipeline.RunTektonPipelineStep,
	}
}

// ImplementationForType Override of default step method
func (cli *RunTektonPipelineStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *RunTektonPipelineStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: tekton pipeline step for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *RunTektonPipelineStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed tekton pipeline for the application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *RunTektonPipelineStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Infof("Run tekton pipeline app %s for branch %s and commit %s", appName, branch, commitID)

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

func (cli *RunTektonPipelineStepImplementation) getRunTektonPipelinesJobConfig(pipelineInfo *model.
	PipelineInfo) *batchv1.Job {
	appName := cli.GetAppName()
	action := "run"
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
			Name:  defaults.RadixPipelineRunEnvironmentVariable,
			Value: pipelineInfo.PipelineArguments.RadixPipelineRun,
		},
		{
			Name:  defaults.RadixPipelineTargetEnvironmentsVariable,
			Value: getTargetEnvironments(pipelineInfo),
		},
		{
			Name:  defaults.LogLevel,
			Value: cli.GetEnv().GetLogLevel(),
		},
	}
	return pipelineUtils.CreateTektonPipelineJob(defaults.RadixPipelineJobRunTektonContainerName, action, pipelineInfo, appName, nil, &envVars)
}

func getTargetEnvironments(pipelineInfo *model.PipelineInfo) string {
	var targetEnvs []string
	for targetEnv, _ := range pipelineInfo.TargetEnvironments {
		targetEnvs = append(targetEnvs, targetEnv)
	}
	targetEnvironments := fmt.Sprintf("(%s)", strings.Join(targetEnvs, ","))
	return targetEnvironments
}
