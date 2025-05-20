package internal

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"path"
	"regexp"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/applicationconfig"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
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
	gitHash, err := getGitHash(pipelineInfo)
	if err != nil {
		return nil, err
	}
	if err = git.ResetGitHead(pipelineInfo.GetGitWorkspace(), gitHash); err != nil {
		return nil, err
	}

	pipelineType := pipelineInfo.GetRadixPipelineType()
	buildContext := model.BuildContext{}

	if pipelineType == v1.BuildDeploy || pipelineType == v1.Build {
		pipelineTargetCommitHash, commitTags, err := getGitAttributes(pipelineInfo)
		if err != nil {
			return nil, err
		}
		pipelineInfo.SetGitAttributes(pipelineTargetCommitHash, commitTags)

		if len(pipelineInfo.PipelineArguments.CommitID) > 0 {
			radixConfigWasChanged, environmentsToBuild, err := cb.analyseSourceRepositoryChanges(pipelineInfo, pipelineTargetCommitHash)
			if err != nil {
				return nil, err
			}
			buildContext.ChangedRadixConfig = radixConfigWasChanged
			buildContext.EnvironmentsToBuild = environmentsToBuild
		} // when commit hash is not provided, build all
	}
	return &buildContext, nil
}

func getGitAttributes(pipelineInfo *model.PipelineInfo) (string, string, error) {
	pipelineTargetCommitHash, commitTags, err := git.GetCommitHashAndTags(pipelineInfo)
	if err != nil {
		return "", "", err
	}
	if err = radixvalidators.GitTagsContainIllegalChars(commitTags); err != nil {
		return "", "", err
	}
	return pipelineTargetCommitHash, commitTags, nil
}

func (cb *contextBuilder) analyseSourceRepositoryChanges(pipelineInfo *model.PipelineInfo, pipelineTargetCommitHash string) (bool, []model.EnvironmentToBuild, error) {
	radixDeploymentCommitHashProvider := commithash.NewProvider(cb.kubeClient, cb.radixClient, pipelineInfo.GetAppName(), pipelineInfo.TargetEnvironments)
	lastCommitHashesForEnvs, err := radixDeploymentCommitHashProvider.GetLastCommitHashesForEnvironments()
	if err != nil {
		return false, nil, err
	}

	changesFromGitRepository, radixConfigWasChanged, err := git.GetChangesFromGitRepository(pipelineInfo.GetGitWorkspace(),
		pipelineInfo.GetRadixConfigBranch(),
		pipelineInfo.GetRadixConfigFileInWorkspace(),
		pipelineTargetCommitHash,
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

func getGitHash(pipelineInfo *model.PipelineInfo) (string, error) {
	// getGitHash return git commit to which the user repository should be reset before parsing sub-pipelines.
	pipelineArgs := pipelineInfo.PipelineArguments
	pipelineType := pipelineInfo.GetRadixPipelineType()
	if pipelineType == v1.ApplyConfig {
		return "", nil
	}

	if pipelineType == v1.Promote {
		sourceRdHashFromAnnotation := pipelineInfo.SourceDeploymentGitCommitHash
		sourceDeploymentGitBranch := pipelineInfo.SourceDeploymentGitBranch
		if sourceRdHashFromAnnotation != "" {
			return sourceRdHashFromAnnotation, nil
		}
		if sourceDeploymentGitBranch == "" {
			log.Info().Msg("Source deployment has no git metadata, skipping sub-pipelines")
			return "", nil
		}
		sourceRdHashFromBranchHead, err := git.GetCommitHashFromHead(pipelineInfo.GetGitWorkspace(), sourceDeploymentGitBranch, "branch")
		if err != nil {
			return "", nil
		}
		return sourceRdHashFromBranchHead, nil
	}

	if pipelineType == v1.Deploy {
		pipelineJobBranch := ""
		re := applicationconfig.GetEnvironmentFromRadixApplication(pipelineInfo.GetRadixApplication(), pipelineArgs.ToEnvironment)
		if re != nil {
			pipelineJobBranch = re.Build.From
		}
		if pipelineJobBranch == "" {
			log.Info().Msgf("Deploy job with no build %s, skipping sub-pipelines.", pipelineInfo.GetGitRefTypeOrDefault())
			return "", nil
		}
		if containsRegex(pipelineJobBranch) {
			log.Info().Msgf("Deploy job with build %s having regex pattern, skipping sub-pipelines.", pipelineInfo.GetGitRefTypeOrDefault())
			return "", nil
		}
		gitHash, err := git.GetCommitHashFromHead(pipelineArgs.GitWorkspace, pipelineJobBranch, "branch")
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}

	if pipelineType == v1.BuildDeploy || pipelineType == v1.Build {
		gitHash, err := git.GetCommitHash(pipelineInfo)
		if err != nil {
			return "", err
		}
		return gitHash, nil
	}
	return "", fmt.Errorf("unknown pipeline type %s", pipelineType)
}

func containsRegex(value string) bool {
	if simpleSentence := regexp.MustCompile(`^[a-zA-Z0-9\s\.\-/]+$`); simpleSentence.MatchString(value) {
		return false
	}
	// Regex value that looks for typical regex special characters
	if specialRegexChars := regexp.MustCompile(`[\[\](){}.*+?^$\\|]`); specialRegexChars.FindStringIndex(value) != nil {
		_, err := regexp.Compile(value)
		return err == nil
	}
	return false
}
