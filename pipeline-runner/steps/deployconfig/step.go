package deployconfig

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/deployconfig/internal"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
)

// DeployConfigStepImplementation Step to deploy RD into environment
type DeployConfigStepImplementation struct {
	stepType               pipeline.StepType
	radixDeploymentWatcher watcher.RadixDeploymentWatcher
	model.DefaultStepImplementation
}

// NewDeployConfigStep Constructor
func NewDeployConfigStep(radixDeploymentWatcher watcher.RadixDeploymentWatcher) model.Step {
	return &DeployConfigStepImplementation{
		stepType:               pipeline.DeployConfigStep,
		radixDeploymentWatcher: radixDeploymentWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployConfigStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	var featureProviders []internal.FeatureProvider
	if pipelineInfo.PipelineArguments.ApplyConfigOptions.DeployExternalDNS {
		featureProviders = append(featureProviders, &internal.ExternalDNSFeatureProvider{})
	}

	handler := internal.NewHandler(pipelineInfo, cli.GetRadixClient(), cli.GetKubeClient(), cli.radixDeploymentWatcher, featureProviders)
	if err := handler.Deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy config: %w", err)
	}

	return nil
}
