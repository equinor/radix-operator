package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
)

// DeployStepImplementation Step to deploy RD into environment
type DeployStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewDeployStep Constructor
func NewDeployStep() model.Step {
	return &DeployStepImplementation{
		stepType: pipeline.DeployStep,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployStepImplementation) SucceededMsg(pipelineInfo *model.PipelineInfo) string {
	return fmt.Sprintf("Succeded: deploy application %s", pipelineInfo.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployStepImplementation) ErrorMsg(pipelineInfo *model.PipelineInfo, err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", pipelineInfo.GetAppName(), err)
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
	appName := pipelineInfo.RadixRegistration.Name
	containerRegistry, err := cli.DefaultStepImplementation.Kubeutil.GetContainerRegistry()
	if err != nil {
		return nil, err
	}

	log.Infof("Deploying app %s", appName)

	radixDeployments, err := deployment.ConstructForTargetEnvironments(
		pipelineInfo.RadixApplication,
		containerRegistry,
		pipelineInfo.PipelineArguments.JobName,
		pipelineInfo.PipelineArguments.ImageTag,
		pipelineInfo.PipelineArguments.Branch,
		pipelineInfo.PipelineArguments.CommitID,
		pipelineInfo.TargetEnvironments)
	if err != nil {
		return nil, fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
	}

	for _, radixDeployment := range radixDeployments {
		deployment, err := deployment.NewDeployment(
			cli.DefaultStepImplementation.Kubeclient,
			cli.DefaultStepImplementation.Radixclient,
			cli.DefaultStepImplementation.PrometheusOperatorClient,
			pipelineInfo.RadixRegistration,
			&radixDeployment)
		if err != nil {
			return nil, err
		}

		err = deployment.Apply()
		if err != nil {
			return nil, fmt.Errorf("Failed to apply radix deployments for app %s. %v", appName, err)
		}
	}

	return radixDeployments, nil
}
