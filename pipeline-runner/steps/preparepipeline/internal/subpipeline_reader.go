package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	defaults2 "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/rs/zerolog/log"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// SubPipelineReader Interface for reading sub-pipeline and tasks
type SubPipelineReader interface {
	ReadPipelineAndTasks(pipelineInfo *model.PipelineInfo, envName string) (bool, string, *pipelinev1.Pipeline, []pipelinev1.Task, error)
}

type subPipelineReader struct{}

// NewSubPipelineReader New instance of the subPipeline reader
func NewSubPipelineReader() SubPipelineReader {
	return &subPipelineReader{}
}

var privateSshFolderMode int32 = 0444

// ReadPipelineAndTasks reads the pipeline and tasks from the specified file path
func (s *subPipelineReader) ReadPipelineAndTasks(pipelineInfo *model.PipelineInfo, envName string) (bool, string, *pipelinev1.Pipeline, []pipelinev1.Task, error) {
	pipelineFilePath, err := getPipelineFilePath(pipelineInfo, "") // TODO - get pipeline for the envName
	if err != nil {
		return false, "", nil, nil, err
	}

	exists, err := fileExists(pipelineFilePath)
	if err != nil {
		return false, "", nil, nil, err
	}
	if !exists {
		log.Info().Msgf("There is no Tekton pipeline file: %s for the environment %s. Skip Tekton pipeline", pipelineFilePath, envName)
		return false, "", nil, nil, nil
	}
	pipeline, err := getPipeline(pipelineFilePath)
	if err != nil {
		return false, "", nil, nil, err
	}
	log.Debug().Msgf("loaded a pipeline with %d tasks", len(pipeline.Spec.Tasks))

	tasks, err := getPipelineTasks(pipelineFilePath, pipeline)
	if err != nil {
		return false, "", nil, nil, err
	}
	log.Debug().Msg("all pipeline tasks found")
	return true, pipelineFilePath, pipeline, tasks, nil
}

func fileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getPipelineTasks(pipelineFilePath string, pipeline *pipelinev1.Pipeline) ([]pipelinev1.Task, error) {
	taskMap, err := getTasks(pipelineFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed get tasks: %w", err)
	}
	if err := hoistEmbeddedTaskSpecs(pipeline, taskMap); err != nil {
		return nil, err
	}
	if len(taskMap) == 0 {
		return nil, errors.New("no tasks found")
	}
	var tasks []pipelinev1.Task
	var validateTaskErrors []error
	for _, pipelineSpecTask := range slices.Concat(pipeline.Spec.Tasks, pipeline.Spec.Finally) {
		task, taskExists := taskMap[pipelineSpecTask.TaskRef.Name]
		if !taskExists {
			validateTaskErrors = append(validateTaskErrors, fmt.Errorf("missing the pipeline task %s, referenced to the task %s", pipelineSpecTask.Name, pipelineSpecTask.TaskRef.Name))
			continue
		}
		validateTaskErrors = append(validateTaskErrors, validation.ValidateTask(&task))
		tasks = append(tasks, task)
	}
	return tasks, errors.Join(validateTaskErrors...)
}

// hoistEmbeddedTaskSpecs converts each inline taskSpec into a regular task and replaces it with a taskRef,
// so embedded tasks get the same validation, substitution and hardening as tasks loaded from task files.
func hoistEmbeddedTaskSpecs(pipeline *pipelinev1.Pipeline, taskMap map[string]pipelinev1.Task) error {
	var errs []error
	hoistedTaskNames := make(map[string]bool)
	for _, pipelineSpecTasks := range [][]pipelinev1.PipelineTask{pipeline.Spec.Tasks, pipeline.Spec.Finally} {
		for i := range pipelineSpecTasks {
			pipelineSpecTask := &pipelineSpecTasks[i]
			if pipelineSpecTask.TaskSpec == nil {
				continue
			}
			if pipelineSpecTask.TaskSpec.IsCustomTask() {
				errs = append(errs, fmt.Errorf("the pipeline task %s defines a custom task %s, which is not supported", pipelineSpecTask.Name, pipelineSpecTask.TaskSpec.Kind))
				continue
			}
			if _, taskExists := taskMap[pipelineSpecTask.Name]; taskExists {
				conflict := "a task file"
				if hoistedTaskNames[pipelineSpecTask.Name] {
					conflict = "another inline taskSpec"
				}
				errs = append(errs, fmt.Errorf("the pipeline task %s defines an inline taskSpec, but a task with this name is already defined in %s", pipelineSpecTask.Name, conflict))
				continue
			}
			task := pipelinev1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pipelineSpecTask.Name,
					Labels:      pipelineSpecTask.TaskSpec.Metadata.Labels,
					Annotations: pipelineSpecTask.TaskSpec.Metadata.Annotations,
				},
				Spec: pipelineSpecTask.TaskSpec.TaskSpec,
			}
			addGitDeployKeyVolume(&task)
			taskMap[task.Name] = task
			hoistedTaskNames[task.Name] = true
			pipelineSpecTask.TaskSpec = nil
			pipelineSpecTask.TaskRef = &pipelinev1.TaskRef{Name: task.Name}
		}
	}
	return errors.Join(errs...)
}

func getPipelineFilePath(pipelineInfo *model.PipelineInfo, pipelineFile string) (string, error) {
	if len(pipelineFile) == 0 {
		pipelineFile = defaults.DefaultPipelineFileName
		log.Debug().Msgf("Tekton pipeline file name is not specified, using the default file name %s", defaults.DefaultPipelineFileName)
	}
	pipelineFile = strings.TrimPrefix(pipelineFile, "/") // Tekton pipeline folder currently is relative to the Radix config file repository folder
	configFolder := filepath.Dir(pipelineInfo.GetRadixConfigFileInWorkspace())
	return filepath.Join(configFolder, pipelineFile), nil
}

func getPipeline(pipelineFileName string) (*pipelinev1.Pipeline, error) {
	pipelineFolder := filepath.Dir(pipelineFileName)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}
	pipelineData, err := os.ReadFile(pipelineFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read the pipeline file %s: %w", pipelineFileName, err)
	}
	pipelineData = substituteRadixSecretNames(pipelineData)
	var pipeline pipelinev1.Pipeline
	err = yaml.Unmarshal(pipelineData, &pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to load the pipeline from the file %s: %w", pipelineFileName, err)
	}
	hotfixForPipelineDefaultParamsWithBrokenValue(&pipeline)
	hotfixForPipelineTasksParamsWithBrokenValue(&pipeline)

	log.Debug().Msgf("loaded pipeline %s", pipelineFileName)
	err = validation.ValidatePipeline(&pipeline)
	if err != nil {
		return nil, err
	}
	return &pipeline, nil
}

func hotfixForPipelineDefaultParamsWithBrokenValue(pipeline *pipelinev1.Pipeline) {
	for ip, p := range pipeline.Spec.Params {
		if p.Default != nil && p.Default.ObjectVal != nil && p.Type == "string" && p.Default.ObjectVal["stringVal"] != "" {
			pipeline.Spec.Params[ip].Default = &pipelinev1.ParamValue{
				Type:      "string",
				StringVal: p.Default.ObjectVal["stringVal"],
			}
		}
	}
}
func hotfixForPipelineTasksParamsWithBrokenValue(pipeline *pipelinev1.Pipeline) {
	for _, pipelineSpecTasks := range [][]pipelinev1.PipelineTask{pipeline.Spec.Tasks, pipeline.Spec.Finally} {
		for it, task := range pipelineSpecTasks {
			for ip, p := range task.Params {
				if p.Value.ObjectVal != nil && p.Value.ObjectVal["type"] == "string" && p.Value.ObjectVal["stringVal"] != "" {
					pipelineSpecTasks[it].Params[ip].Value = pipelinev1.ParamValue{
						Type:      "string",
						StringVal: p.Value.ObjectVal["stringVal"],
					}
				}
			}
		}
	}
}

func getTasks(pipelineFilePath string) (map[string]pipelinev1.Task, error) {
	pipelineFolder := filepath.Dir(pipelineFilePath)
	if _, err := os.Stat(pipelineFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing pipeline folder: %s", pipelineFolder)
	}

	fileNameList, err := filepath.Glob(filepath.Join(pipelineFolder, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan pipeline folder %s: %w", pipelineFolder, err)
	}
	taskMap := make(map[string]pipelinev1.Task)
	for _, fileName := range fileNameList {
		if strings.EqualFold(fileName, pipelineFilePath) {
			continue
		}
		fileData, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to read the file %s: %w", fileName, err)
		}
		fileData = substituteRadixSecretNames(fileData)

		task := pipelinev1.Task{}
		err = yaml.Unmarshal(fileData, &task)
		if err != nil {
			return nil, fmt.Errorf("failed to read data from the file %s: %w", fileName, err)
		}
		if !taskIsValid(&task) {
			log.Debug().Msgf("skip the file %s - not a Tekton task", fileName)
			continue
		}
		addGitDeployKeyVolume(&task)
		taskMap[task.Name] = task
	}
	return taskMap, nil
}

func substituteRadixSecretNames(fileData []byte) []byte {
	fileData = []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixBuildSecretsSource, defaults.SubstitutionRadixBuildSecretsTarget))
	return []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixGitDeployKeySource, defaults.SubstitutionRadixGitDeployKeyTarget))
}

func addGitDeployKeyVolume(task *pipelinev1.Task) {
	task.Spec.Volumes = append(task.Spec.Volumes, v1.Volume{
		Name: defaults.SubstitutionRadixGitDeployKeyTarget,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  defaults2.GitPrivateKeySecretName,
				DefaultMode: &privateSshFolderMode,
			},
		},
	})
}

func taskIsValid(task *pipelinev1.Task) bool {
	return strings.HasPrefix(task.APIVersion, "tekton.dev/") && task.Kind == "Task" && len(task.ObjectMeta.Name) > 1
}
