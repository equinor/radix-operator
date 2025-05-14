package internal

import (
	"fmt"
	"path"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type ContextBuilder interface {
	GetBuildContext(pipelineInfo *model.PipelineInfo) (*model.BuildContext, error)
}

type contextBuilder struct {
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

func NewContextBuilder(kubeClient kubernetes.Interface, radixClient radixclient.Interface) ContextBuilder {
	return &contextBuilder{
		kubeClient:  kubeClient,
		radixClient: radixClient,
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
	buildContext := model.BuildContext{}
	if len(pipelineInfo.PipelineArguments.CommitID) > 0 {
		radixConfigWasChanged, environmentsToBuild, err := cb.analyseSourceRepositoryChanges(pipelineInfo)
		if err != nil {
			return nil, err
		}
		buildContext.ChangedRadixConfig = radixConfigWasChanged
		buildContext.EnvironmentsToBuild = environmentsToBuild
	} // when commit hash is not provided, build all

	return &buildContext, nil
}

func (cb *contextBuilder) analyseSourceRepositoryChanges(pipelineInfo *model.PipelineInfo) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(cb.kubeClient, cb.radixClient, pipelineInfo.GetAppName(), pipelineInfo.TargetEnvironments)
	lastCommitHashesForEnvs, err := radixDeploymentCommitHashProvider.GetLastCommitHashesForEnvironments()
	if err != nil {
		return false, nil, err
	}

	changesFromGitRepository, radixConfigWasChanged, err := git.GetChangesFromGitRepository(pipelineInfo.GetGitWorkspace(),
		pipelineInfo.GetRadixConfigBranch(),
		pipelineInfo.GetRadixConfigFileInWorkspace(),
		pipelineInfo.GitCommitHash,
		lastCommitHashesForEnvs)
	if err != nil {
		return false, nil, err
	}

	environmentsToBuild := cb.getEnvironmentsToBuild(pipelineInfo, changesFromGitRepository)
	return radixConfigWasChanged, environmentsToBuild, nil
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
