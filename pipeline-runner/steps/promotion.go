package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
)

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

	return nil
}
