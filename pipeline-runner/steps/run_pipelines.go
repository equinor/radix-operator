package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	internaltekton "github.com/equinor/radix-operator/pipeline-runner/internal/tekton"
	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RunPipelinesStepImplementation Step to run Tekton pipelines
type RunPipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	jobWaiter internalwait.JobCompletionWaiter
}

// NewRunPipelinesStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewRunPipelinesStep(jobWaiter internalwait.JobCompletionWaiter) model.Step {
	return &RunPipelinesStepImplementation{
		stepType:  pipeline.RunPipelinesStep,
		jobWaiter: jobWaiter,
	}
}

func (step *RunPipelinesStepImplementation) Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface, rr *v1.RadixRegistration) {
	step.DefaultStepImplementation.Init(kubeclient, radixclient, kubeutil, prometheusOperatorClient, rr)
	if step.jobWaiter == nil {
		step.jobWaiter = internalwait.NewJobCompletionWaiter(kubeclient)
	}
}

// ImplementationForType Override of default step method
func (step *RunPipelinesStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *RunPipelinesStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: run pipelines step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *RunPipelinesStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed run pipelines for the application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *RunPipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.PrepareBuildContext != nil && len(pipelineInfo.PrepareBuildContext.EnvironmentSubPipelinesToRun) == 0 {
		log.Info().Msg("There is no configured sub-pipelines. Skip the step.")
		return nil
	}
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash
	appName := step.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	log.Info().Msgf("Run pipelines app %s for branch %s and commit %s", appName, branch, commitID)

	job := step.getRunTektonPipelinesJobConfig(pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(ctx, step.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Info().Msgf("Apply job (%s) to run Tekton pipeline %s", job.Name, appName)
	job, err := step.GetKubeclient().BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return step.jobWaiter.Wait(job)
}

func (step *RunPipelinesStepImplementation) getRunTektonPipelinesJobConfig(pipelineInfo *model.
PipelineInfo) *batchv1.Job {
	appName := step.GetAppName()
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
			Name:  defaults.LogLevel,
			Value: pipelineInfo.PipelineArguments.LogLevel,
		},
		{
			Name:  defaults.RadixReservedAppDNSAliasesEnvironmentVariable,
			Value: maps.ToString(pipelineInfo.PipelineArguments.DNSConfig.ReservedAppDNSAliases),
		},
		{
			Name:  defaults.RadixReservedDNSAliasesEnvironmentVariable,
			Value: strings.Join(pipelineInfo.PipelineArguments.DNSConfig.ReservedDNSAliases, ","),
		},
	}
	return internaltekton.CreateActionPipelineJob(defaults.RadixPipelineJobRunPipelinesContainerName, action, pipelineInfo, appName, nil, &envVars)
}
