package promote

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/steps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromoteStepImplementation Step to promote deployment to another environment,
// or inside environment
type PromoteStepImplementation struct {
	stepType steps.StepType
	model.DefaultStepImplementation
}

// NewPromoteStep Constructor
func NewPromoteStep() model.Step {
	return &PromoteStepImplementation{
		stepType: steps.PromoteStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PromoteStepImplementation) ImplementationForType() steps.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *PromoteStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Successful promotion for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PromoteStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Promotion failed for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PromoteStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	var radixDeployment *v1.RadixDeployment

	// Get radix application from cluster as promote step run as single step
	radixApplication, err := cli.GetRadixclient().RadixV1().RadixApplications(utils.GetAppNamespace(cli.GetAppName())).Get(ctx, cli.GetAppName(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	log.Ctx(ctx).Info().Msgf("Promoting %s for application %s from %s to %s", pipelineInfo.PipelineArguments.DeploymentName, cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.ToEnvironment)
	err = areArgumentsValid(pipelineInfo.PipelineArguments)
	if err != nil {
		return err
	}

	fromNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment)
	toNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.ToEnvironment)

	_, err = cli.GetKubeclient().CoreV1().Namespaces().Get(ctx, fromNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingFromEnvironment(pipelineInfo.PipelineArguments.FromEnvironment)
	}

	_, err = cli.GetKubeclient().CoreV1().Namespaces().Get(ctx, toNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingToEnvironment(pipelineInfo.PipelineArguments.ToEnvironment)
	}

	rd, err := cli.GetRadixclient().RadixV1().RadixDeployments(fromNs).Get(ctx, pipelineInfo.PipelineArguments.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return NonExistingDeployment(pipelineInfo.PipelineArguments.DeploymentName)
	}

	radixDeployment = rd.DeepCopy()
	radixDeployment.Name = utils.GetDeploymentName(cli.GetAppName(), pipelineInfo.PipelineArguments.ToEnvironment, pipelineInfo.PipelineArguments.ImageTag)

	if radixDeployment.GetAnnotations() == nil {
		radixDeployment.ObjectMeta.Annotations = make(map[string]string)
	}
	if _, isRestored := radixDeployment.Annotations[kube.RestoredStatusAnnotation]; isRestored {
		// RA-817: Promotion reuses annotation - RD get inactive status
		radixDeployment.Annotations[kube.RestoredStatusAnnotation] = ""
	}
	radixDeployment.Annotations[kube.RadixDeploymentPromotedFromDeploymentAnnotation] = rd.GetName()
	radixDeployment.Annotations[kube.RadixDeploymentPromotedFromEnvironmentAnnotation] = pipelineInfo.PipelineArguments.FromEnvironment

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Labels[kube.RadixEnvLabel] = pipelineInfo.PipelineArguments.ToEnvironment
	radixDeployment.Labels[kube.RadixJobNameLabel] = pipelineInfo.PipelineArguments.JobName
	radixDeployment.Spec.Environment = pipelineInfo.PipelineArguments.ToEnvironment

	err = mergeWithRadixApplication(ctx, radixApplication, radixDeployment, pipelineInfo.PipelineArguments.ToEnvironment, pipelineInfo.DeployEnvironmentComponentImages[pipelineInfo.PipelineArguments.ToEnvironment])
	if err != nil {
		return err
	}

	err = radixvalidators.CanRadixDeploymentBeInserted(radixDeployment)
	if err != nil {
		return err
	}

	if _, err := cli.GetRadixclient().RadixV1().RadixDeployments(toNs).Create(ctx, radixDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func areArgumentsValid(arguments model.PipelineArguments) error {
	if arguments.FromEnvironment == "" {
		return EmptyArgument("From environment")
	}

	if arguments.ToEnvironment == "" {
		return EmptyArgument("To environment")
	}

	if arguments.DeploymentName == "" {
		return EmptyArgument("Deployment name")
	}

	if arguments.JobName == "" {
		return EmptyArgument("Job name")
	}

	if arguments.ImageTag == "" {
		return EmptyArgument("Image tag")
	}

	return nil
}

func mergeWithRadixApplication(ctx context.Context, radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string, componentImages pipeline.DeployComponentImages) error {
	defaultEnvVars := getDefaultEnvVarsFromRadixDeployment(radixDeployment)
	if err := mergeComponentsWithRadixApplication(ctx, radixConfig, radixDeployment, environment, defaultEnvVars, componentImages); err != nil {
		return err
	}

	if err := mergeJobComponentsWithRadixApplication(ctx, radixConfig, radixDeployment, environment, defaultEnvVars, componentImages); err != nil {
		return err
	}

	return nil
}

func mergeJobComponentsWithRadixApplication(ctx context.Context, radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string, defaultEnvVars v1.EnvVarsMap, componentImages pipeline.DeployComponentImages) error {
	newEnvJobs, err := deployment.
		NewJobComponentsBuilder(radixConfig, environment, componentImages, defaultEnvVars, nil).
		JobComponents(ctx)
	if err != nil {
		return err
	}
	newEnvJobsMap := make(map[string]v1.RadixDeployJobComponent)
	for _, job := range newEnvJobs {
		newEnvJobsMap[job.Name] = job
	}

	for idx, job := range radixDeployment.Spec.Jobs {
		newEnvJob, found := newEnvJobsMap[job.Name]
		if !found {
			return NonExistingComponentName(radixConfig.GetName(), job.Name)
		}
		// Environment variables, SecretRefs are taken from current configuration
		newEnvJob.Secrets = job.Secrets
		newEnvJob.Image = job.Image
		newEnvJob.Runtime = job.Runtime
		radixDeployment.Spec.Jobs[idx] = newEnvJob
	}

	return nil
}

func mergeComponentsWithRadixApplication(ctx context.Context, radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string, defaultEnvVars v1.EnvVarsMap, componentImages pipeline.DeployComponentImages) error {
	newEnvComponents, err := deployment.GetRadixComponentsForEnv(ctx, radixConfig, environment, componentImages, defaultEnvVars, nil)
	if err != nil {
		return err
	}

	newEnvComponentsMap := make(map[string]v1.RadixDeployComponent)
	for _, component := range newEnvComponents {
		newEnvComponentsMap[component.Name] = component
	}

	for idx, component := range radixDeployment.Spec.Components {
		newEnvComponent, found := newEnvComponentsMap[component.Name]
		if !found {
			return NonExistingComponentName(radixConfig.GetName(), component.Name)
		}
		// Environment variables, SecretRefs are taken from current configuration
		newEnvComponent.Secrets = component.Secrets
		newEnvComponent.Image = component.Image
		newEnvComponent.Runtime = component.Runtime
		radixDeployment.Spec.Components[idx] = newEnvComponent
	}

	return nil
}

func getDefaultEnvVarsFromRadixDeployment(radixDeployment *v1.RadixDeployment) v1.EnvVarsMap {
	envVarsMap := make(v1.EnvVarsMap)
	gitCommitHash := internal.GetGitCommitHashFromDeployment(radixDeployment)
	if gitCommitHash != "" {
		envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
	}
	if gitTags, ok := radixDeployment.Annotations[kube.RadixGitTagsAnnotation]; ok {
		envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags
	}
	return envVarsMap
}
