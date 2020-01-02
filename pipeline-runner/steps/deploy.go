package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
)

// DeployStepImplementation Step to deploy RD into environment
type DeployStepImplementation struct {
	stepType         pipeline.StepType
	namespaceWatcher kube.NamespaceWatcher
	model.DefaultStepImplementation
}

// NewDeployStep Constructor
func NewDeployStep(namespaceWatcher kube.NamespaceWatcher) model.Step {
	return &DeployStepImplementation{
		stepType:         pipeline.DeployStep,
		namespaceWatcher: namespaceWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	if !pipelineInfo.BranchIsMapped {
		// Do nothing
		return fmt.Errorf("Skip deploy step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
	}

	_, err := cli.deploy(pipelineInfo)
	return err
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployStepImplementation) deploy(pipelineInfo *model.PipelineInfo) ([]v1.RadixDeployment, error) {
	appName := cli.GetAppName()
	log.Infof("Deploying app %s", appName)
	radixDeployments := []v1.RadixDeployment{}

	for _, env := range pipelineInfo.RadixApplication.Spec.Environments {
		if !deployment.DeployToEnvironment(env, pipelineInfo.TargetEnvironments) {
			continue
		}

		radixDeployment, err := deployment.ConstructForTargetEnvironment(
			pipelineInfo.RadixApplication,
			pipelineInfo.ContainerRegistry,
			pipelineInfo.PipelineArguments.JobName,
			pipelineInfo.PipelineArguments.ImageTag,
			pipelineInfo.PipelineArguments.Branch,
			pipelineInfo.PipelineArguments.CommitID,
			pipelineInfo.ComponentImages,
			env.Name)

		if err != nil {
			return nil, fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
		}

		deployment, err := deployment.NewDeployment(
			cli.GetKubeclient(),
			cli.GetKubeutil(),
			cli.GetRadixclient(),
			cli.GetPrometheusOperatorClient(),
			cli.GetRegistration(),
			&radixDeployment)
		if err != nil {
			return nil, err
		}

		err = cli.namespaceWatcher.WaitFor(utils.GetEnvironmentNamespace(cli.GetAppName(), env.Name))
		if err != nil {
			return nil, fmt.Errorf("Failed to get environment namespace, %s, for app %s. %v", env, appName, err)
		}

		err = deployment.Apply()
		if err != nil {
			return nil, fmt.Errorf("Failed to apply radix deployments for app %s. %v", appName, err)
		}

		radixDeployments = append(radixDeployments, radixDeployment)
	}

	return radixDeployments, nil
}
