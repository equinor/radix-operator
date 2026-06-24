package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	defaults2 "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/rs/zerolog/log"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1 "k8s.io/api/core/v1"
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
	if len(taskMap) == 0 {
		return nil, fmt.Errorf("no tasks found: %w", err)
	}
	var tasks []pipelinev1.Task
	var validateTaskErrors []error
	for _, pipelineSpecTask := range pipeline.Spec.Tasks {
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
	for it, task := range pipeline.Spec.Tasks {
		for ip, p := range task.Params {
			if p.Value.ObjectVal != nil && p.Value.ObjectVal["type"] == "string" && p.Value.ObjectVal["stringVal"] != "" {
				pipeline.Spec.Tasks[it].Params[ip].Value = pipelinev1.ParamValue{
					Type:      "string",
					StringVal: p.Value.ObjectVal["stringVal"],
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
		fileData = []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixBuildSecretsSource, defaults.SubstitutionRadixBuildSecretsTarget))
		fileData = []byte(strings.ReplaceAll(string(fileData), defaults.SubstitutionRadixGitDeployKeySource, defaults.SubstitutionRadixGitDeployKeyTarget))

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
