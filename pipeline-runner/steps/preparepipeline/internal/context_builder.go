package internal

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type ContextBuilder interface {
	GetBuildContext(pipelineInfo *model.PipelineInfo) (*model.BuildContext, error)
}

type contextBuilder struct {
	kubeUtil *kube.Kube
}

func NewContextBuilder(kubeUtil *kube.Kube) ContextBuilder {
	return &contextBuilder{
		kubeUtil: kubeUtil,
	}
}

// ComponentHasChangedSource checks if the component has changed source
func ComponentHasChangedSource(envName string, component v1.RadixCommonComponent, changedFolders []string) bool {
	image := component.GetImageForEnvironment(envName)
	if len(image) > 0 {
		return false
	}
	environmentConfig := component.GetEnvironmentConfigByName(envName)
	if !component.GetEnabledForEnvironmentConfig(environmentConfig) {
		return false
	}

	componentSource := component.GetSourceForEnvironment(envName)
	sourceFolder := cleanPathAndSurroundBySlashes(componentSource.Folder)
	if path.Dir(sourceFolder) == path.Dir("/") && len(changedFolders) > 0 {
		return true // for components with the repository root as a 'src' - changes in any repository sub-folders are considered also as the component changes
	}

	for _, folder := range changedFolders {
		if strings.HasPrefix(cleanPathAndSurroundBySlashes(folder), sourceFolder) {
			return true
		}
	}
	return false
}

func cleanPathAndSurroundBySlashes(dir string) string {
	if !strings.HasSuffix(dir, "/") {
		dir = fmt.Sprintf("%s/", dir)
	}
	dir = fmt.Sprintf("%s/", path.Dir(dir))
	if !strings.HasPrefix(dir, "/") {
		return fmt.Sprintf("/%s", dir)
	}
	return dir
}

// GetBuildContext Prepare build context
func (cb *contextBuilder) GetBuildContext(pipelineInfo *model.PipelineInfo) (*model.BuildContext, error) {
	if len(pipelineInfo.PipelineArguments.CommitID) == 0 {
		return nil, nil
	}

	radixConfigWasChanged, environmentsToBuild, err := cb.analyseSourceRepositoryChanges(pipelineInfo)
	if err != nil {
		return nil, err
	}

	buildContext := model.BuildContext{
		ChangedRadixConfig:  radixConfigWasChanged,
		EnvironmentsToBuild: environmentsToBuild,
	}

	return &buildContext, nil
}

func (cb *contextBuilder) analyseSourceRepositoryChanges(pipelineInfo *model.PipelineInfo) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(cb.kubeUtil) //cb.kubeClient, cb.radixClient, pipelineInfo.GetAppName(), pipelineInfo.TargetEnvironments

	var changedRadixConfig bool
	envChangedDirs := make(map[string][]string)
	for _, envName := range pipelineInfo.TargetEnvironments {
		envCommitInfo, err := radixDeploymentCommitHashProvider.GetLastCommitHashForEnvironment(context.Background(), pipelineInfo.GetAppName(), envName)
		if err != nil {
			return false, nil, err
		}

		changedPaths, envChangedRadixConfig, err := git.GetChangesFromGitRepository(pipelineInfo.GetGitWorkspace(), pipelineInfo.GetRadixConfigBranch(), pipelineInfo.GetRadixConfigFileInWorkspace(), pipelineInfo.GitCommitHash, envCommitInfo.CommitHash)
		if err != nil {
			return false, nil, err
		}

		changedRadixConfig = changedRadixConfig || envChangedRadixConfig
		envChangedDirs[envName] = changedPaths
	}

	environmentsToBuild := cb.getEnvironmentsToBuild(pipelineInfo, envChangedDirs)
	return changedRadixConfig, environmentsToBuild, nil
}

func (cb *contextBuilder) getEnvironmentsToBuild(pipelineInfo *model.PipelineInfo, changesFromGitRepository map[string][]string) []model.EnvironmentToBuild {
	var environmentsToBuild []model.EnvironmentToBuild
	for envName, changedFolders := range changesFromGitRepository {
		var componentsWithChangedSource []string
		for _, radixComponent := range pipelineInfo.GetRadixApplication().Spec.Components {
			if ComponentHasChangedSource(envName, &radixComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
			}
		}
		for _, radixJobComponent := range pipelineInfo.GetRadixApplication().Spec.Jobs {
			if ComponentHasChangedSource(envName, &radixJobComponent, changedFolders) {
				componentsWithChangedSource = append(componentsWithChangedSource, radixJobComponent.GetName())
			}
		}
		environmentsToBuild = append(environmentsToBuild, model.EnvironmentToBuild{
			Environment: envName,
			Components:  componentsWithChangedSource,
		})
	}
	return environmentsToBuild
}
