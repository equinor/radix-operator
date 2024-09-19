package preparepipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	internalgit "github.com/equinor/radix-operator/pipeline-runner/internal/git"
	internaltekton "github.com/equinor/radix-operator/pipeline-runner/internal/tekton"
	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobUtil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PreparePipelinesStepImplementation Step to prepare radixconfig and Tekton pipelines
type PreparePipelinesStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	jobWaiter internalwait.JobCompletionWaiter
}

// NewPreparePipelinesStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewPreparePipelinesStep(jobWaiter internalwait.JobCompletionWaiter) model.Step {
	return &PreparePipelinesStepImplementation{
		stepType:  pipeline.PreparePipelinesStep,
		jobWaiter: jobWaiter,
	}
}

func (step *PreparePipelinesStepImplementation) Init(ctx context.Context, kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface, rr *radixv1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeclient, radixclient, kubeutil, prometheusOperatorClient, rr)
	if step.jobWaiter == nil {
		step.jobWaiter = internalwait.NewJobCompletionWaiter(ctx, kubeclient)
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
func (cli *PreparePipelinesStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.PipelineArguments.CommitID
	appName := cli.GetAppName()
	namespace := utils.GetAppNamespace(appName)
	logPipelineInfo(ctx, pipelineInfo.Definition.Type, appName, branch, commitID)

	if pipelineInfo.IsPipelineType(radixv1.Promote) {
		sourceDeploymentGitCommitHash, sourceDeploymentGitBranch, err := cli.getSourceDeploymentGitInfo(ctx, appName, pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.DeploymentName)
		if err != nil {
			return err
		}
		pipelineInfo.SourceDeploymentGitCommitHash = sourceDeploymentGitCommitHash
		pipelineInfo.SourceDeploymentGitBranch = sourceDeploymentGitBranch
	}
	job := cli.getPreparePipelinesJobConfig(pipelineInfo)

	// When debugging pipeline there will be no RJ
	if !pipelineInfo.PipelineArguments.Debug {
		ownerReference, err := jobUtil.GetOwnerReferenceOfJob(ctx, cli.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
		if err != nil {
			return err
		}

		job.OwnerReferences = ownerReference
	}

	log.Ctx(ctx).Info().Msgf("Apply job (%s) to copy radixconfig to configmap for app %s and prepare Tekton pipeline", job.Name, appName)
	job, err := cli.GetKubeclient().BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return cli.jobWaiter.Wait(job)
}

func logPipelineInfo(ctx context.Context, pipelineType radixv1.RadixPipelineType, appName, branch, commitID string) {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(fmt.Sprintf("Prepare pipeline %s for the app %s", pipelineType, appName))
	if len(branch) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the branch %s", branch))
	}
	if len(branch) > 0 {
		stringBuilder.WriteString(fmt.Sprintf(", the commit %s", commitID))
	}
	log.Ctx(ctx).Info().Msg(stringBuilder.String())
}

func (cli *PreparePipelinesStepImplementation) getPreparePipelinesJobConfig(pipelineInfo *model.PipelineInfo) *batchv1.Job {
	appName := cli.GetAppName()
	registration := cli.GetRegistration()
	configBranch := applicationconfig.GetConfigBranch(registration)

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
			Name:  defaults.RadixConfigBranchEnvironmentVariable,
			Value: configBranch,
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
			Value: pipelineInfo.PipelineArguments.LogLevel,
		},
		{
			Name:  defaults.RadixGithubWebhookCommitId,
			Value: getWebhookCommitID(pipelineInfo),
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
	initContainers := git.CloneInitContainersWithContainerName(registration.Spec.CloneURL, configBranch, git.CloneConfigContainerName, internalgit.CloneConfigFromPipelineArgs(pipelineInfo.PipelineArguments), false)
	return internaltekton.CreateActionPipelineJob(defaults.RadixPipelineJobPreparePipelinesContainerName, action, pipelineInfo, appName, initContainers, &envVars)
}

func getWebhookCommitID(pipelineInfo *model.PipelineInfo) string {
	if pipelineInfo.IsPipelineType(radixv1.BuildDeploy) {
		return pipelineInfo.PipelineArguments.CommitID
	}
	return ""
}

func (cli *PreparePipelinesStepImplementation) getSourceDeploymentGitInfo(ctx context.Context, appName, sourceEnvName, sourceDeploymentName string) (string, string, error) {
	ns := utils.GetEnvironmentNamespace(appName, sourceEnvName)
	rd, err := cli.GetKubeutil().GetRadixDeployment(ctx, ns, sourceDeploymentName)
	if err != nil {
		return "", "", err
	}
	gitHash := internal.GetGitCommitHashFromDeployment(rd)
	gitBranch := rd.Annotations[kube.RadixBranchAnnotation]
	return gitHash, gitBranch, err
}
