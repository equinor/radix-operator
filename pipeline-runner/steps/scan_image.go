package steps

import (
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
)

// ScanImageImplementation Step to scan image for vulnerabilities
type ScanImageImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewScanImageStep Constructor
func NewScanImageStep() model.Step {
	return &ScanImageImplementation{
		stepType: pipeline.ScanImageStep,
	}
}

// ImplementationForType Override of default step method
func (cli *ScanImageImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *ScanImageImplementation) SucceededMsg() string {
	return fmt.Sprintf("Image scan successful for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *ScanImageImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Image scan successful for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *ScanImageImplementation) Run(pipelineInfo *model.PipelineInfo) error {

	return nil
}
