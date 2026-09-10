package validation_test

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func taskRefPipelineTask(name, taskRefName string) pipelinev1.PipelineTask {
	return pipelinev1.PipelineTask{Name: name, TaskRef: &pipelinev1.TaskRef{Name: taskRefName}}
}

func taskSpecPipelineTask(name string) pipelinev1.PipelineTask {
	return pipelinev1.PipelineTask{
		Name: name,
		TaskSpec: &pipelinev1.EmbeddedTask{
			TaskSpec: pipelinev1.TaskSpec{Steps: []pipelinev1.Step{{Name: "step1", Image: "alpine"}}},
		},
	}
}

func TestValidatePipeline(t *testing.T) {
	specs := []struct {
		name          string
		tasks         []pipelinev1.PipelineTask
		finally       []pipelinev1.PipelineTask
		expectedError string
	}{
		{
			name:  "task with taskRef is valid",
			tasks: []pipelinev1.PipelineTask{taskRefPipelineTask("task1", "hello")},
		},
		{
			name:          "task with inline taskSpec is rejected",
			tasks:         []pipelinev1.PipelineTask{taskSpecPipelineTask("task1")},
			expectedError: "invalid task #1 task1: each Task within a Pipeline must have a valid name and a taskRef",
		},
		{
			name:    "finally with inline taskSpec is valid",
			tasks:   []pipelinev1.PipelineTask{taskRefPipelineTask("task1", "hello")},
			finally: []pipelinev1.PipelineTask{taskSpecPipelineTask("finally1")},
		},
		{
			name:    "finally with taskRef is valid",
			tasks:   []pipelinev1.PipelineTask{taskRefPipelineTask("task1", "hello")},
			finally: []pipelinev1.PipelineTask{taskRefPipelineTask("finally1", "hello")},
		},
		{
			name:          "no tasks",
			expectedError: "missing tasks in the pipeline",
		},
		{
			name:          "task with neither taskRef nor taskSpec",
			tasks:         []pipelinev1.PipelineTask{{Name: "task1"}},
			expectedError: "invalid task #1 task1: each Task within a Pipeline must have a valid name and a taskRef",
		},
		{
			name:          "task without a name",
			tasks:         []pipelinev1.PipelineTask{taskRefPipelineTask("", "hello")},
			expectedError: "each Task within a Pipeline must have a valid name and a taskRef",
		},
		{
			name:          "finally with neither taskRef nor taskSpec",
			tasks:         []pipelinev1.PipelineTask{taskRefPipelineTask("task1", "hello")},
			finally:       []pipelinev1.PipelineTask{{Name: "finally1"}},
			expectedError: "invalid finally task #1 'finally1'",
		},
		{
			name: "task with both taskRef and taskSpec",
			tasks: []pipelinev1.PipelineTask{{
				Name:     "task1",
				TaskRef:  &pipelinev1.TaskRef{Name: "hello"},
				TaskSpec: taskSpecPipelineTask("task1").TaskSpec,
			}},
			expectedError: "must have either a taskRef or a taskSpec, not both",
		},
		{
			name:  "finally with both taskRef and taskSpec",
			tasks: []pipelinev1.PipelineTask{taskRefPipelineTask("task1", "hello")},
			finally: []pipelinev1.PipelineTask{{
				Name:     "finally1",
				TaskRef:  &pipelinev1.TaskRef{Name: "hello"},
				TaskSpec: taskSpecPipelineTask("finally1").TaskSpec,
			}},
			expectedError: "must have either a taskRef or a taskSpec, not both",
		},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			pipeline := pipelinev1.Pipeline{
				Spec: pipelinev1.PipelineSpec{Tasks: spec.tasks, Finally: spec.finally},
			}

			err := validation.ValidatePipeline(&pipeline)

			if spec.expectedError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), spec.expectedError)
		})
	}
}
