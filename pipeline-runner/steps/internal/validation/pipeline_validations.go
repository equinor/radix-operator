package validation

import (
	"errors"
	"fmt"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ValidatePipeline Validate Pipeline
func ValidatePipeline(pipeline *pipelinev1.Pipeline) error {
	var validationErrors []error

	validationErrors = append(validationErrors, validatePipelineTasks(pipeline)...)

	return errors.Join(validationErrors...)
}

func validatePipelineTasks(pipeline *pipelinev1.Pipeline) []error {
	var validationErrors []error
	if len(pipeline.Spec.Tasks) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("missing tasks in the pipeline %s", pipeline.Name))
	}
	for i, pipelineSpecTask := range pipeline.Spec.Tasks {
		if len(pipelineSpecTask.Name) == 0 || pipelineSpecTask.TaskRef == nil {
			validationErrors = append(validationErrors,
				fmt.Errorf("invalid task #%d %s: each Task within a Pipeline must have a valid name and a taskRef.\n"+
					"https://tekton.dev/docs/pipelines/pipelines/#adding-tasks-to-the-pipeline",
					i+1, pipelineSpecTask.Name))
		}
	}
	return validationErrors
}
