package internal

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

var (
	ErrMissingGitCommitHash = errors.New("missing target git commit hash")
)

type ContextBuilder interface {
	GetBuildContext(ctx context.Context, pipelineInfo *model.PipelineInfo, repo git.Repository) (*model.BuildContext, error)
}

type contextBuilder struct{}

// NewContextBuilder creates a new ContexBuilder.
// If logger argument is nil, the default global logger is used
func NewContextBuilder() ContextBuilder {
	return &contextBuilder{}
}

// GetBuildContext Prepare build context
func (cb *contextBuilder) GetBuildContext(ctx context.Context, pipelineInfo *model.PipelineInfo, repo git.Repository) (*model.BuildContext, error) {
	if len(pipelineInfo.TargetEnvironments) == 0 {
		return nil, nil
	}

	if len(pipelineInfo.GitCommitHash) == 0 {
		return nil, ErrMissingGitCommitHash
	}

	var environmentsToBuild []model.EnvironmentToBuild
	for _, targetEnv := range pipelineInfo.TargetEnvironments {
		envCommitHash := internal.GetGitCommitHashFromDeployment(targetEnv.ActiveRadixDeployment)

		if len(envCommitHash) > 0 {
			commitExist, err := repo.CommitExists(envCommitHash)
			if err != nil {
				return nil, fmt.Errorf("failed to check if commit from active deployment exists: %w", err)
			}
			if !commitExist {
				log.Ctx(ctx).Info().Msgf("Commit from active deployment in environment %s was not found in the repository.", targetEnv.Environment)
				envCommitHash = ""
			}
		}

		changedFiles, err := repo.DiffCommits(envCommitHash, pipelineInfo.GitCommitHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get changes in git: %w", err)
		}

		environmentsToBuild = append(environmentsToBuild, model.EnvironmentToBuild{
			Environment: targetEnv.Environment,
			Components:  getComponentsToBuild(pipelineInfo.GetRadixApplication(), targetEnv.Environment, changedFiles),
		})
	}

	buildContext := model.BuildContext{
		EnvironmentsToBuild: environmentsToBuild,
	}

	return &buildContext, nil
}

func getComponentsToBuild(ra *radixv1.RadixApplication, envName string, changedFiles git.DiffEntries) []string {
	var componentsWithChangedSource []string

	for _, radixComponent := range ra.Spec.Components {
		if componentHasChangedSource(envName, &radixComponent, changedFiles) {
			componentsWithChangedSource = append(componentsWithChangedSource, radixComponent.GetName())
		}
	}
	for _, radixJobComponent := range ra.Spec.Jobs {
		if componentHasChangedSource(envName, &radixJobComponent, changedFiles) {
			componentsWithChangedSource = append(componentsWithChangedSource, radixJobComponent.GetName())
		}
	}
	return componentsWithChangedSource
}

// componentHasChangedSource checks if the component has changed source
func componentHasChangedSource(envName string, component radixv1.RadixCommonComponent, changedFiles git.DiffEntries) bool {
	if len(changedFiles) == 0 {
		return false
	}

	if len(component.GetImageForEnvironment(envName)) > 0 {
		return false
	}

	if !component.GetEnabledForEnvironmentConfig(component.GetEnvironmentConfigByName(envName)) {
		return false
	}

	componentSource := component.GetSourceForEnvironment(envName)

	cleanedSourceFolder := cleanPathForPrefixComparison(componentSource.Folder)
	if cleanedSourceFolder == "/" {
		return true // for components with the repository root as a 'src' - changes in any repository sub-folders are considered also as the component changes
	}

	dockerFilePath := filepath.Clean(componentSource.DockerfileName)
	if !filepath.IsAbs(dockerFilePath) {
		dockerFilePath = filepath.Join(cleanedSourceFolder, dockerFilePath)
	}

	for _, entry := range changedFiles {

		// Check if the changed entry is within the component's source folder
		if dir := entryDir(entry); strings.HasPrefix(dir, cleanedSourceFolder) {
			return true
		}

		// Check if the changed entry is the Dockerfile itself
		if !entry.IsDir {
			cleanedEntry := path.Join("/", filepath.Clean(entry.Name))
			if cleanedEntry == dockerFilePath {
				return true
			}
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

func entryDir(entry git.DiffEntry) string {
	if entry.IsDir {
		return cleanPathForPrefixComparison(entry.Name)
	}
	return cleanPathForPrefixComparison(path.Dir(entry.Name))
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
