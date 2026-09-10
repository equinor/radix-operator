package internal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const helloTaskFile = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: hello
spec:
  steps:
    - name: say-hello
      image: alpine
      script: echo hello
`

func writeSubPipeline(t *testing.T, pipelineYaml string, taskFiles map[string]string) *model.PipelineInfo {
	t.Helper()
	workspace := t.TempDir()
	tektonFolder := filepath.Join(workspace, "tekton")
	require.NoError(t, os.MkdirAll(tektonFolder, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tektonFolder, "pipeline.yaml"), []byte(pipelineYaml), 0644))
	for fileName, content := range taskFiles {
		require.NoError(t, os.WriteFile(filepath.Join(tektonFolder, fileName), []byte(content), 0644))
	}
	return &model.PipelineInfo{PipelineArguments: model.PipelineArguments{
		GitWorkspace:    workspace,
		RadixConfigFile: "radixconfig.yaml",
	}}
}

func findTask(tasks []pipelinev1.Task, name string) (pipelinev1.Task, bool) {
	for _, task := range tasks {
		if task.Name == name {
			return task, true
		}
	}
	return pipelinev1.Task{}, false
}

func Test_ReadPipelineAndTasks_HoistsInlineFinallyTaskSpec(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-inline-finally
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        steps:
          - name: show
            image: alpine
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	exists, _, pipeline, tasks, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.NoError(t, err)
	require.True(t, exists)
	require.Len(t, tasks, 2, "both the referenced task and the hoisted inline task are returned")

	hoisted, found := findTask(tasks, "finally-task")
	require.True(t, found, "the inline taskSpec is returned as a task")
	require.Len(t, hoisted.Spec.Steps, 1)
	assert.Equal(t, "show", hoisted.Spec.Steps[0].Name)

	require.Len(t, pipeline.Spec.Finally, 1)
	assert.Nil(t, pipeline.Spec.Finally[0].TaskSpec, "the inline taskSpec is replaced")
	require.NotNil(t, pipeline.Spec.Finally[0].TaskRef)
	assert.Equal(t, "finally-task", pipeline.Spec.Finally[0].TaskRef.Name)
}

func Test_ReadPipelineAndTasks_ValidatesInlineFinallyTaskSpec(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-invalid-inline-finally
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        steps:
          - name: show-secrets
            image: alpine
            env:
              - name: SECRET
                valueFrom:
                  secretKeyRef:
                    name: some-other-secret
                    key: password
            script: echo $SECRET
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err, "an inline taskSpec referencing a non-Radix secret is rejected")
	assert.Contains(t, err.Error(), "finally-task")
}

func Test_ReadPipelineAndTasks_SubstitutesRadixSecretNamesInInlineTaskSpec(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-build-secrets
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        steps:
          - name: show
            image: alpine
            envFrom:
              - secretRef:
                  name: $(radix.build-secrets)
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, tasks, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.NoError(t, err)
	hoisted, found := findTask(tasks, "finally-task")
	require.True(t, found)
	require.Len(t, hoisted.Spec.Steps[0].EnvFrom, 1)
	assert.Equal(t, "build-secrets", hoisted.Spec.Steps[0].EnvFrom[0].SecretRef.Name)
}

func Test_ReadPipelineAndTasks_ResolvesFinallyTaskRef(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-finally-taskref
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-hello
      taskRef:
        name: hello
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, tasks, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.NoError(t, err)
	assert.Len(t, tasks, 2, "a task referenced from finally is resolved and returned")
}

func Test_ReadPipelineAndTasks_RejectsUnresolvedFinallyTaskRef(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-missing-finally-taskref
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-missing
      taskRef:
        name: does-not-exist
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func Test_ReadPipelineAndTasks_RejectsInlineTaskSpecNameCollision(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-colliding-inline-task
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: hello
      taskSpec:
        steps:
          - name: show
            image: alpine
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already defined in a task file")
}

func Test_ReadPipelineAndTasks_RejectsDuplicateInlineTaskSpecNames(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-duplicate-inline-tasks
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: duplicated
      taskSpec:
        steps:
          - name: show
            image: alpine
            script: echo one
    - name: duplicated
      taskSpec:
        steps:
          - name: show
            image: alpine
            script: echo two
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "another inline taskSpec")
}

func Test_ReadPipelineAndTasks_RejectsInlineTaskSpecInTasks(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-inline-task
spec:
  tasks:
    - name: inline-task
      taskSpec:
        steps:
          - name: show
            image: alpine
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, nil)

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err, "an inline taskSpec is only supported in the finally block")
	assert.Contains(t, err.Error(), "must have a valid name and a taskRef")
}

func Test_ReadPipelineAndTasks_CarriesInlineTaskSpecMetadata(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-inline-metadata
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        metadata:
          labels:
            azure.workload.identity/use: "true"
          annotations:
            azure.workload.identity/skip-containers: show
        steps:
          - name: show
            image: alpine
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, tasks, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.NoError(t, err)
	hoisted, found := findTask(tasks, "finally-task")
	require.True(t, found)
	assert.Equal(t, "true", hoisted.Labels["azure.workload.identity/use"], "inline metadata labels are carried over")
	assert.Equal(t, "show", hoisted.Annotations["azure.workload.identity/skip-containers"], "inline metadata annotations are carried over")
}

func Test_ReadPipelineAndTasks_RejectsIllegalLabelInInlineTaskSpec(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-illegal-label
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        metadata:
          labels:
            some-illegal-label: "true"
        steps:
          - name: show
            image: alpine
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err, "an inline taskSpec is subject to the same label rules as a task file")
	assert.Contains(t, err.Error(), "some-illegal-label")
}

func Test_ReadPipelineAndTasks_AddsGitDeployKeyVolumeToInlineTaskSpec(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-git-deploy-key
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-task
      taskSpec:
        steps:
          - name: show
            image: alpine
            volumeMounts:
              - name: $(radix.git-deploy-key)
                mountPath: /root/.ssh
            script: echo done
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, tasks, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.NoError(t, err)
	hoisted, found := findTask(tasks, "finally-task")
	require.True(t, found)
	assert.Equal(t, "radix-git-deploy-key", hoisted.Spec.Steps[0].VolumeMounts[0].Name, "the git deploy key placeholder is substituted")
	require.Len(t, hoisted.Spec.Volumes, 1)
	assert.Equal(t, "radix-git-deploy-key", hoisted.Spec.Volumes[0].Name, "the git deploy key volume is added to inline tasks too")
}

func Test_ReadPipelineAndTasks_NoTaskFilesAndNoInlineSpecs(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-without-tasks
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, nil)

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tasks found")
}

func Test_ReadPipelineAndTasks_RejectsInlineCustomTask(t *testing.T) {
	pipelineYaml := `
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: pipeline-with-custom-task
spec:
  tasks:
    - name: use-hello
      taskRef:
        name: hello
  finally:
    - name: finally-custom
      taskSpec:
        apiVersion: example.dev/v0
        kind: Example
        spec:
          field: value
`
	pipelineInfo := writeSubPipeline(t, pipelineYaml, map[string]string{"hello.yaml": helloTaskFile})

	_, _, _, _, err := internal.NewSubPipelineReader().ReadPipelineAndTasks(pipelineInfo, "dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom task")
}
