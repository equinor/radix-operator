package steps

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EmptyArgument Argument by name cannot be empty
func EmptyArgument(argumentName string) error {
	return fmt.Errorf("%s cannot be empty", argumentName)
}

// NonExistingFromEnvironment From environment does not exist
func NonExistingFromEnvironment(environment string) error {
	return fmt.Errorf("non existing from environment %s", environment)
}

// NonExistingToEnvironment From environment does not exist
func NonExistingToEnvironment(environment string) error {
	return fmt.Errorf("non existing to environment %s", environment)
}

// NonExistingDeployment Deployment wasn't found
func NonExistingDeployment(deploymentName string) error {
	return fmt.Errorf("non existing deployment %s", deploymentName)
}

// NonExistingComponentName Component by name was not found
func NonExistingComponentName(appName, componentName string) error {
	return fmt.Errorf("unable to get application component %s for app %s", componentName, appName)
}

// PromoteStepImplementation Step to promote deployment to another environment,
// or inside environment
type PromoteStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPromoteStep Constructor
func NewPromoteStep() model.Step {
	return &PromoteStepImplementation{
		stepType: pipeline.PromoteStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PromoteStepImplementation) ImplementationForType() pipeline.StepType {
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
func (cli *PromoteStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	var radixDeployment *v1.RadixDeployment

	// Get radix application from cluster as promote step run as single step
	radixApplication, err := cli.GetRadixclient().RadixV1().RadixApplications(utils.GetAppNamespace(cli.GetAppName())).Get(context.TODO(), cli.GetAppName(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	log.Infof("Promoting %s for application %s from %s to %s", pipelineInfo.PipelineArguments.DeploymentName, cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.ToEnvironment)
	err = areArgumentsValid(pipelineInfo.PipelineArguments)
	if err != nil {
		return err
	}

	fromNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment)
	toNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.ToEnvironment)

	_, err = cli.GetKubeclient().CoreV1().Namespaces().Get(context.TODO(), fromNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingFromEnvironment(pipelineInfo.PipelineArguments.FromEnvironment)
	}

	_, err = cli.GetKubeclient().CoreV1().Namespaces().Get(context.TODO(), toNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingToEnvironment(pipelineInfo.PipelineArguments.ToEnvironment)
	}

	rd, err := cli.GetRadixclient().RadixV1().RadixDeployments(fromNs).Get(context.TODO(), pipelineInfo.PipelineArguments.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return NonExistingDeployment(pipelineInfo.PipelineArguments.DeploymentName)
	}

	radixDeployment = rd.DeepCopy()
	radixDeployment.Name = utils.GetDeploymentName(cli.GetAppName(), pipelineInfo.PipelineArguments.ToEnvironment, pipelineInfo.PipelineArguments.ImageTag)

	if _, isRestored := radixDeployment.Annotations[kube.RestoredStatusAnnotation]; isRestored {
		// RA-817: Promotion reuses annotation - RD get inactive status
		radixDeployment.Annotations[kube.RestoredStatusAnnotation] = ""
	}

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Labels[kube.RadixEnvLabel] = pipelineInfo.PipelineArguments.ToEnvironment
	radixDeployment.Labels[kube.RadixJobNameLabel] = pipelineInfo.PipelineArguments.JobName
	radixDeployment.Spec.Environment = pipelineInfo.PipelineArguments.ToEnvironment

	err = mergeWithRadixApplication(radixApplication, radixDeployment, pipelineInfo.PipelineArguments.ToEnvironment)
	if err != nil {
		return err
	}

	isValid, err := radixvalidators.CanRadixDeploymentBeInserted(cli.GetRadixclient(), radixDeployment)
	if !isValid {
		return err
	}

	if _, err := cli.GetRadixclient().RadixV1().RadixDeployments(toNs).Create(context.TODO(), radixDeployment, metav1.CreateOptions{}); err != nil {
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

func mergeWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	if err := mergeComponentsWithRadixApplication(radixConfig, radixDeployment, environment); err != nil {
		return err
	}

	if err := mergeJobComponentsWithRadixApplication(radixConfig, radixDeployment, environment); err != nil {
		return err
	}

	return nil
}

func mergeJobComponentsWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	newEnvJobs := deployment.
		NewJobComponentsBuilder(radixConfig, environment, make(map[string]pipeline.ComponentImage)).
		JobComponents()

	newEnvJobsMap := make(map[string]v1.RadixDeployJobComponent)
	for _, job := range newEnvJobs {
		newEnvJobsMap[job.Name] = job
	}

	for idx, job := range radixDeployment.Spec.Jobs {
		newEnvJob, found := newEnvJobsMap[job.Name]
		if !found {
			return NonExistingComponentName(radixConfig.GetName(), job.Name)
		}

		newEnvJob.Secrets = job.Secrets
		newEnvJob.Image = job.Image
		radixDeployment.Spec.Jobs[idx] = newEnvJob
	}

	return nil
}

func mergeComponentsWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	newEnvComponents := deployment.GetRadixComponentsForEnv(radixConfig, environment, make(map[string]pipeline.ComponentImage))

	newEnvComponentsMap := make(map[string]v1.RadixDeployComponent)
	for _, component := range newEnvComponents {
		newEnvComponentsMap[component.Name] = component
	}

	for idx, component := range radixDeployment.Spec.Components {
		newEnvComponent, found := newEnvComponentsMap[component.Name]
		if !found {
			return NonExistingComponentName(radixConfig.GetName(), component.Name)
		}

		newEnvComponent.Secrets = component.Secrets
		newEnvComponent.SecretRefs = component.SecretRefs
		newEnvComponent.Image = component.Image
		radixDeployment.Spec.Components[idx] = newEnvComponent
	}

	return nil
}
