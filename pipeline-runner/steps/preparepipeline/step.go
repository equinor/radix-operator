package preparepipeline

import (
	"context"
	"fmt"
	"strings"

	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
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

func (cli *PreparePipelinesStepImplementation) Init(ctx context.Context, kubeClient kubernetes.Interface, radixClient radixclient.Interface, kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, tektonClient tektonclient.Interface, rr *radixv1.RadixRegistration) {
	cli.DefaultStepImplementation.Init(ctx, kubeClient, radixClient, kubeUtil, prometheusOperatorClient, tektonClient, rr)
	if cli.jobWaiter == nil {
		cli.jobWaiter = internalwait.NewJobCompletionWaiter(ctx, kubeClient)
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
	logPipelineInfo(ctx, pipelineInfo.Definition.Type, appName, branch, commitID)

	if pipelineInfo.IsPipelineType(radixv1.Promote) {
		sourceDeploymentGitCommitHash, sourceDeploymentGitBranch, err := cli.getSourceDeploymentGitInfo(ctx, appName, pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.DeploymentName)
		if err != nil {
			return err
		}
		pipelineInfo.SourceDeploymentGitCommitHash = sourceDeploymentGitCommitHash
		pipelineInfo.SourceDeploymentGitBranch = sourceDeploymentGitBranch
	}

	pipelineCtx := NewPipelineContext(cli.GetKubeClient(), cli.GetRadixClient(), cli.GetTektonClient(), pipelineInfo)

	radixApplication, err := pipelineCtx.LoadRadixAppConfig()
	if err != nil {
		return err
	}

	pipelineInfo.SetRadixApplication(radixApplication)
	targetEnvironments, err := internal.GetPipelineTargetEnvironments(pipelineInfo)
	if err != nil {
		return err
	}
	pipelineCtx.SetPipelineTargetEnvironments(targetEnvironments)

	buildContext, err := pipelineCtx.GetBuildContext()
	if err != nil {
		return err
	}
	pipelineInfo.SetBuildContext(buildContext)

	buildContext.EnvironmentSubPipelinesToRun, err = pipelineCtx.GetEnvironmentSubPipelinesToRun()
	if err != nil {
		return err
	}
	return nil
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

func (cli *PreparePipelinesStepImplementation) getSourceDeploymentGitInfo(ctx context.Context, appName, sourceEnvName, sourceDeploymentName string) (string, string, error) {
	ns := utils.GetEnvironmentNamespace(appName, sourceEnvName)
	rd, err := cli.GetKubeUtil().GetRadixDeployment(ctx, ns, sourceDeploymentName)
	if err != nil {
		return "", "", err
	}
	gitHash := internal.GetGitCommitHashFromDeployment(rd)
	gitBranch := rd.Annotations[kube.RadixBranchAnnotation]
	return gitHash, gitBranch, err
}
