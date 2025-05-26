package internal

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

var (
	ErrMissingGitCommitHash = errors.New("missing target git commit hash")
)

type ContextBuilder interface {
	GetBuildContext(pipelineInfo *model.PipelineInfo, repo git.Repository) (*model.BuildContext, error)
}

type contextBuilder struct {
	kubeUtil *kube.Kube
}

func NewContextBuilder(kubeUtil *kube.Kube) ContextBuilder {
	return &contextBuilder{
		kubeUtil: kubeUtil,
	}
}

// GetBuildContext Prepare build context
func (cb *contextBuilder) GetBuildContext(pipelineInfo *model.PipelineInfo, repo git.Repository) (*model.BuildContext, error) {
	if len(pipelineInfo.GitCommitHash) == 0 {
		return nil, ErrMissingGitCommitHash
	}

	if len(pipelineInfo.TargetEnvironments) == 0 {
		return nil, nil
	}

	radixDeploymentCommitHashProvider := commithash.NewProvider(cb.kubeUtil) //cb.kubeClient, cb.radixClient, pipelineInfo.GetAppName(), pipelineInfo.TargetEnvironments
	var environmentsToBuild []model.EnvironmentToBuild

	for _, envName := range pipelineInfo.TargetEnvironments {
		// TODO: replace with GetActiveRadixDeployment in steps/internal package
		envCommitInfo, err := radixDeploymentCommitHashProvider.GetLastCommitHashForEnvironment(context.Background(), pipelineInfo.GetAppName(), envName)
		if err != nil {
			return nil, err
		}

		changedFiles, err := repo.DiffCommits(envCommitInfo.CommitHash, pipelineInfo.GitCommitHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get changes in git: %w", err)
		}

		environmentsToBuild = append(environmentsToBuild, model.EnvironmentToBuild{
			Environment: envName,
			Components:  getComponentsToBuild(pipelineInfo.GetRadixApplication(), envName, distinctDirs(changedFiles)),
		})
	}

	buildContext := model.BuildContext{
		ChangedRadixConfig:  false,
		EnvironmentsToBuild: environmentsToBuild,
	}

	return &buildContext, nil
}

func getComponentsToBuild(ra *v1.RadixApplication, envName string, changedDirs []string) []string {
	var componentsWithChangedSource []string

	for _, radixComponent := range ra.Spec.Components {
		if componentHasChangedSource(envName, &radixComponent, changedDirs) {
			componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
		}
	}
	for _, radixJobComponent := range ra.Spec.Jobs {
		if componentHasChangedSource(envName, &radixJobComponent, changedDirs) {
			componentsWithChangedSource = append(componentsWithChangedSource, radixJobComponent.GetName())
		}
	}
	return componentsWithChangedSource
}

// componentHasChangedSource checks if the component has changed source
func componentHasChangedSource(envName string, component v1.RadixCommonComponent, changedFolders []string) bool {
	if len(changedFolders) == 0 {
		return false
	}

	if len(component.GetImageForEnvironment(envName)) > 0 {
		return false
	}

	if !component.GetEnabledForEnvironmentConfig(component.GetEnvironmentConfigByName(envName)) {
		return false
	}

	cleanedSourceFolder := cleanPathForPrefixComparison(component.GetSourceForEnvironment(envName).Folder)
	if cleanedSourceFolder == "/" {
		return true // for components with the repository root as a 'src' - changes in any repository sub-folders are considered also as the component changes
	}

	for _, folder := range changedFolders {
		if strings.HasPrefix(cleanPathForPrefixComparison(folder), cleanedSourceFolder) {
			return true
		}
	}
	return false
}

func cleanPathForPrefixComparison(dir string) string {
	outDir := path.Join("/", dir)
	if !strings.HasSuffix(outDir, "/") {
		outDir = outDir + "/"
	}
	return outDir
}

// Returns list of distinct directories
func distinctDirs(s git.DiffEntries) []string {
	allDirs := slice.Map(s, func(e git.DiffEntry) string {
		if e.IsDir {
			return e.Name
		}
		return path.Dir(e.Name)
	})
	slices.Sort(allDirs)
	return slices.Compact(allDirs)
}
