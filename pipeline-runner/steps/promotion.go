package steps

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NonExistingFromEnvironment From environment does not exist
func NonExistingFromEnvironment(environment string) error {
	return fmt.Errorf("Non existing from environment %s", environment)
}

// NonExistingToEnvironment From environment does not exist
func NonExistingToEnvironment(environment string) error {
	return fmt.Errorf("Non existing to environment %s", environment)
}

// NonExistingDeployment Deployment wasn't found
func NonExistingDeployment(deploymentName string) error {
	return fmt.Errorf("Non existing deployment %s", deploymentName)
}

// NonExistingComponentName Component by name was not found
func NonExistingComponentName(appName, componentName string) error {
	return fmt.Errorf("Unable to get application component %s for app %s", componentName, appName)
}

// PromoteStepImplementation Step to apply RA
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
	return fmt.Sprintf("Succeded: promoted application %s", cli.DefaultStepImplementation.Registration.Name)
}

// ErrorMsg Override of default step method
func (cli *PromoteStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to promote application %s. Error: %v", cli.DefaultStepImplementation.Registration.Name, err)
}

// Run Override of default step method
func (cli *PromoteStepImplementation) Run(pipelineInfo model.PipelineInfo) error {
	var radixDeployment *v1.RadixDeployment

	fromNs := utils.GetEnvironmentNamespace(pipelineInfo.GetAppName(), pipelineInfo.FromEnvironment)
	toNs := utils.GetEnvironmentNamespace(pipelineInfo.GetAppName(), pipelineInfo.ToEnvironment)

	_, err := cli.DefaultStepImplementation.Kubeclient.CoreV1().Namespaces().Get(fromNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingFromEnvironment(pipelineInfo.FromEnvironment)
	}

	_, err = cli.DefaultStepImplementation.Kubeclient.CoreV1().Namespaces().Get(toNs, metav1.GetOptions{})
	if err != nil {
		return NonExistingFromEnvironment(pipelineInfo.ToEnvironment)
	}

	log.Infof("Promoting %s from %s to %s", pipelineInfo.GetAppName(), pipelineInfo.FromEnvironment, pipelineInfo.ToEnvironment)
	radixDeployment, err = cli.DefaultStepImplementation.Radixclient.RadixV1().RadixDeployments(fromNs).Get(pipelineInfo.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return NonExistingDeployment(pipelineInfo.DeploymentName)
	}

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Labels[kube.RadixEnvLabel] = pipelineInfo.ToEnvironment
	radixDeployment.Labels[kube.RadixJobNameLabel] = pipelineInfo.JobName
	radixDeployment.Spec.Environment = pipelineInfo.ToEnvironment

	err = mergeWithRadixApplication(pipelineInfo.RadixApplication, radixDeployment, pipelineInfo.ToEnvironment)
	if err != nil {
		return err
	}

	isValid, err := radixvalidators.CanRadixDeploymentBeInserted(cli.DefaultStepImplementation.Radixclient, radixDeployment)
	if !isValid {
		return err
	}

	radixDeployment, err = cli.DefaultStepImplementation.Radixclient.RadixV1().RadixDeployments(toNs).Create(radixDeployment)
	if err != nil {
		return err
	}

	return nil
}

func mergeWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	for index, comp := range radixDeployment.Spec.Components {
		raComp := getComponentConfig(radixConfig, comp.Name)
		if raComp == nil {
			return NonExistingComponentName(radixConfig.GetName(), comp.Name)
		}

		environmentVariables := getEnvironmentVariables(raComp, environment)
		radixDeployment.Spec.Components[index].EnvironmentVariables = environmentVariables
	}

	return nil
}

func getComponentConfig(radixConfig *v1.RadixApplication, componentName string) *v1.RadixComponent {
	for _, comp := range radixConfig.Spec.Components {
		if strings.EqualFold(comp.Name, componentName) {
			return &comp
		}
	}

	return nil
}

func getEnvironmentVariables(componentConfig *v1.RadixComponent, environment string) v1.EnvVarsMap {
	for _, environmentConfig := range componentConfig.EnvironmentConfig {
		if strings.EqualFold(environmentConfig.Environment, environment) {
			return environmentConfig.Variables
		}
	}

	return v1.EnvVarsMap{}
}
