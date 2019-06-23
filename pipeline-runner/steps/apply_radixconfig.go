package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
)

// ApplyConfigStepImplementation Step to apply RA
type ApplyConfigStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewApplyConfigStep Constructor
func NewApplyConfigStep() model.Step {
	return &ApplyConfigStepImplementation{
		stepType: pipeline.ApplyConfigStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ApplyConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *ApplyConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Applied config for application %s", cli.DefaultStepImplementation.Registration.Name)
}

// ErrorMsg Override of default step method
func (cli *ApplyConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to apply config for application %s. Error: %v", cli.DefaultStepImplementation.Registration.Name, err)
}

// Run Override of default step method
func (cli *ApplyConfigStepImplementation) Run(pipelineInfo model.PipelineInfo) error {
	applicationConfig, err := application.NewApplicationConfig(cli.DefaultStepImplementation.Kubeclient, cli.DefaultStepImplementation.Radixclient, cli.DefaultStepImplementation.Registration, cli.DefaultStepImplementation.ApplicationConfig)
	if err != nil {
		return err
	}

	err = applicationConfig.ApplyConfigToApplicationNamespace()
	if err != nil {
		return err
	}

	return nil
}
