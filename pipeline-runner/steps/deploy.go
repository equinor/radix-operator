package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
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
	err := cli.deploy(pipelineInfo)
	return err
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployStepImplementation) deploy(pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	log.Infof("Deploying app %s", appName)

	if !pipelineInfo.BranchIsMapped {
		// Do nothing
		return fmt.Errorf("Skip deploy step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
	}

	for env, shouldDeploy := range pipelineInfo.TargetEnvironments {
		if !shouldDeploy {
			continue
		}

		err := cli.deployToEnv(appName, env, pipelineInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cli *DeployStepImplementation) deployToEnv(appName, env string, pipelineInfo *model.PipelineInfo) error {
	radixDeployment, err := deployment.ConstructForTargetEnvironment(
		pipelineInfo.RadixApplication,
		pipelineInfo.PipelineArguments.JobName,
		pipelineInfo.PipelineArguments.ImageTag,
		pipelineInfo.PipelineArguments.Branch,
		pipelineInfo.PipelineArguments.CommitID,
		pipelineInfo.ComponentImages,
		env)

	if err != nil {
		return fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
	}

	deployment, err := deployment.NewDeployment(
		cli.GetKubeclient(),
		cli.GetKubeutil(),
		cli.GetRadixclient(),
		cli.GetPrometheusOperatorClient(),
		cli.GetRegistration(),
		&radixDeployment)
	if err != nil {
		return err
	}

	err = cli.namespaceWatcher.WaitFor(utils.GetEnvironmentNamespace(cli.GetAppName(), env))
	if err != nil {
		return fmt.Errorf("Failed to get environment namespace, %s, for app %s. %v", env, appName, err)
	}

	err = deployment.Apply()
	if err != nil {
		return fmt.Errorf("Failed to apply radix deployment for app %s to environment %s. %v", appName, env, err)
	}

	return nil
}
