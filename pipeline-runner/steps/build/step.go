package build

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	internalbuild "github.com/equinor/radix-operator/pipeline-runner/internal/jobs/build"
	internalwait "github.com/equinor/radix-operator/pipeline-runner/internal/wait"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobutil "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type BuildJobFactory func(useBuildKit bool) internalbuild.Interface

// BuildStepImplementation Step to build docker image
type BuildStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
	jobWaiter       internalwait.JobCompletionWaiter
	buildJobFactory BuildJobFactory
}

type Option func(step *BuildStepImplementation)

func WithBuildJobFactory(factory BuildJobFactory) Option {
	return func(step *BuildStepImplementation) {
		step.buildJobFactory = factory
	}
}

func defaultBuildJobFactory(useBuildKit bool) internalbuild.Interface {
	if useBuildKit {
		return internalbuild.NewBuildKit()
	}

	return internalbuild.NewACR()
}

// NewBuildStep Constructor.
// jobWaiter is optional and will be set by Init(...) function if nil.
func NewBuildStep(jobWaiter internalwait.JobCompletionWaiter, options ...Option) model.Step {
	step := &BuildStepImplementation{
		stepType:        pipeline.BuildStep,
		jobWaiter:       jobWaiter,
		buildJobFactory: defaultBuildJobFactory,
	}

	for _, o := range options {
		o(step)
	}

	return step
}

func (step *BuildStepImplementation) Init(ctx context.Context, kubeclient kubernetes.Interface, radixclient radixclient.Interface, kubeutil *kube.Kube, prometheusOperatorClient monitoring.Interface, rr *v1.RadixRegistration) {
	step.DefaultStepImplementation.Init(ctx, kubeclient, radixclient, kubeutil, prometheusOperatorClient, rr)
	if step.jobWaiter == nil {
		step.jobWaiter = internalwait.NewJobCompletionWaiter(ctx, kubeclient)
	}
}

// ImplementationForType Override of default step method
func (step *BuildStepImplementation) ImplementationForType() pipeline.StepType {
	return step.stepType
}

// SucceededMsg Override of default step method
func (step *BuildStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: build step for application %s", step.GetAppName())
}

// ErrorMsg Override of default step method
func (step *BuildStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to build application %s. Error: %v", step.GetAppName(), err)
}

// Run Override of default step method
func (step *BuildStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	branch := pipelineInfo.PipelineArguments.Branch
	commitID := pipelineInfo.GitCommitHash

	if len(pipelineInfo.TargetEnvironments) == 0 {
		log.Ctx(ctx).Info().Msgf("Skip build step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
		return nil
	}

	if len(pipelineInfo.BuildComponentImages) == 0 {
		log.Ctx(ctx).Info().Msgf("No component in app %s requires building", step.GetAppName())
		return nil
	}

	log.Ctx(ctx).Info().Msgf("Building app %s for branch %s and commit %s", step.GetAppName(), branch, commitID)

	if err := step.validateBuildSecrets(pipelineInfo); err != nil {
		return err
	}

	jobs := step.getBuildJobs(pipelineInfo)
	namespace := utils.GetAppNamespace(step.GetAppName())
	return step.applyBuildJobs(ctx, pipelineInfo, jobs, namespace)
}

func (step *BuildStepImplementation) getBuildJobs(pipelineInfo *model.PipelineInfo) []batchv1.Job {
	rr := step.GetRegistration()
	var secrets []string
	if pipelineInfo.RadixApplication.Spec.Build != nil {
		secrets = pipelineInfo.RadixApplication.Spec.Build.Secrets
	}
	imagesToBuild := slices.Concat(maps.Values(pipelineInfo.BuildComponentImages)...)
	return step.buildJobFactory(pipelineInfo.IsUsingBuildKit()).
		GetJobs(
			pipelineInfo.IsUsingBuildCache(),
			pipelineInfo.PipelineArguments,
			rr.Spec.CloneURL,
			pipelineInfo.GitCommitHash,
			pipelineInfo.GitTags,
			imagesToBuild,
			secrets,
		)
}

func (step *BuildStepImplementation) applyBuildJobs(ctx context.Context, pipelineInfo *model.PipelineInfo, jobs []batchv1.Job, namespace string) error {
	ownerReference, err := step.getJobOwnerReferences(ctx, pipelineInfo, namespace)
	if err != nil {
		return err
	}

	g := errgroup.Group{}
	for _, job := range jobs {
		g.Go(func() error {
			logger := log.Ctx(ctx).With().Str("job", job.Name).Logger()
			job.OwnerReferences = ownerReference
			jobDescription := step.getJobDescription(&job)
			logger.Info().Msgf("Apply %s", jobDescription)
			createdJob, err := step.GetKubeclient().BatchV1().Jobs(namespace).Create(context.Background(), &job, metav1.CreateOptions{})
			if err != nil {
				logger.Error().Err(err).Msgf("failed %s", jobDescription)
				return err
			}
			return step.jobWaiter.Wait(createdJob)
		})
	}
	return g.Wait()
}

func (step *BuildStepImplementation) getJobOwnerReferences(ctx context.Context, pipelineInfo *model.PipelineInfo, namespace string) ([]metav1.OwnerReference, error) {
	// When debugging pipeline there will be no RJ
	if pipelineInfo.PipelineArguments.Debug {
		return nil, nil
	}
	ownerReference, err := jobutil.GetOwnerReferenceOfJob(ctx, step.GetRadixclient(), namespace, pipelineInfo.PipelineArguments.JobName)
	if err != nil {
		return nil, err
	}
	return ownerReference, nil
}

func (step *BuildStepImplementation) getJobDescription(job *batchv1.Job) string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("the job %s to build", job.Name))
	if componentName, ok := job.GetLabels()[kube.RadixComponentLabel]; ok {
		builder.WriteString(fmt.Sprintf(" the component %s", componentName))
	} else {
		builder.WriteString(" components")
	}
	if envName, ok := job.GetLabels()[kube.RadixEnvLabel]; ok {
		builder.WriteString(fmt.Sprintf(" in the environment %s", envName))
	}
	return builder.String()
}

func (*BuildStepImplementation) validateBuildSecrets(pipelineInfo *model.PipelineInfo) error {
	if pipelineInfo.RadixApplication.Spec.Build == nil || len(pipelineInfo.RadixApplication.Spec.Build.Secrets) == 0 {
		return nil
	}

	if pipelineInfo.BuildSecret == nil {
		return errors.New("build secrets has not been set")
	}

	for _, secretName := range pipelineInfo.RadixApplication.Spec.Build.Secrets {
		if secretValue, ok := pipelineInfo.BuildSecret.Data[secretName]; !ok || strings.EqualFold(string(secretValue), defaults.BuildSecretDefaultData) {
			return fmt.Errorf("build secret %s has not been set", secretName)
		}
	}

	return nil
}
