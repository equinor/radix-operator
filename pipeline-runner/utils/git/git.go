package git

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/rs/zerolog/log"
)

// ResetGitHead alters HEAD of the git repository on file system to point to commitHashString
func ResetGitHead(gitWorkspace, commitHashString string) error {
	r, err := git.PlainOpen(gitWorkspace)
	if err != nil {
		return err
	}
	log.Debug().Msgf("opened repositoryPath %s", gitWorkspace)

	worktree, err := r.Worktree()
	if err != nil {
		return err
	}

	commitHash := plumbing.NewHash(commitHashString)
	err = worktree.Reset(&git.ResetOptions{
		Commit: commitHash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return err
	}
	log.Debug().Msgf("reset HEAD to %s", commitHashString)
	return nil
}

// GetCommitHashAndTags gets target commit hash and tags from GitHub repository
func GetCommitHashAndTags(gitWorkspace, webhookCommitId, branchName string) (string, string, error) {
	targetCommitHash, err := GetCommitHash(gitWorkspace, webhookCommitId, branchName)
	if err != nil {
		return "", "", err
	}

	gitTags, err := getGitCommitTags(gitWorkspace, targetCommitHash)
	if err != nil {
		return "", "", err
	}
	return targetCommitHash, gitTags, nil
}

func getGitDir(gitWorkspace string) string {
	return gitWorkspace + "/.git"
}

// GetCommitHashFromHead returns the commit hash for the HEAD of branchName in gitDir
func GetCommitHashFromHead(gitWorkspace string, branchName string) (string, error) {
	gitDir := getGitDir(gitWorkspace)
	r, err := git.PlainOpen(gitDir)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("opened gitDir %s", gitDir)

	// Get branchName hash
	commitHash, err := getBranchCommitHash(r, branchName)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("resolved branch %s", branchName)

	hashBytesString := hex.EncodeToString(commitHash[:])
	return hashBytesString, nil
}

// getGitAffectedResourcesBetweenCommits returns the list of folders, where files were affected after beforeCommitHash (not included) till targetCommitHash commit (included)
func getGitAffectedResourcesBetweenCommits(gitWorkspace, configBranch, configFile string, triggeredFromWebhook bool, targetCommitString, beforeCommitString string) ([]string, bool, error) {
	gitDir := getGitDir(gitWorkspace)
	targetCommitHash, err := getTargetCommitHash(triggeredFromWebhook, beforeCommitString, targetCommitString)
	if err != nil {
		return nil, false, err
	}
	repository, currentBranch, err := getRepository(gitDir)
	if err != nil {
		return nil, false, err
	}
	beforeCommitHash, err := getBeforeCommitHash(beforeCommitString, repository)
	if (err != nil && err != io.EOF) && beforeCommitHash == nil {
		return nil, false, err
	}
	beforeCommit, err := repository.CommitObject(*beforeCommitHash)
	if err != nil {
		return nil, false, err
	}

	if strings.EqualFold(beforeCommitHash.String(), targetCommitString) { // targetCommit is the very first commit in the repo
		return getChangedFoldersOfCommitFiles(beforeCommit, configBranch, currentBranch, configFile)
	}

	targetCommit, err := repository.CommitObject(*targetCommitHash)
	if (err != nil && err != io.EOF) && targetCommit == nil {
		return nil, false, err
	}
	return getChangedFoldersFromTargetCommitTillExclusiveBeforeCommit(beforeCommit, targetCommit, configBranch, currentBranch, configFile)
}

func getChangedFoldersFromTargetCommitTillExclusiveBeforeCommit(targetCommit *object.Commit, beforeCommit *object.Commit, configBranch string, currentBranch string, configFile string) ([]string, bool, error) {
	beforeTree, err := beforeCommit.Tree()
	if err != nil {
		return nil, false, err
	}
	targetTree, err := targetCommit.Tree()
	if err != nil {
		return nil, false, err
	}
	changes, err := beforeTree.Diff(targetTree)
	if err != nil {
		return nil, false, err
	}
	changedFolderNamesMap := make(map[string]bool)
	changedConfigFile := false
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, false, err
		}
		fileName := change.To.Name
		if action == merkletrie.Delete {
			fileName = change.From.Name
		} else if action == merkletrie.Modify && change.To.Name != change.From.Name {
			appendFolderToMap(changedFolderNamesMap, &changedConfigFile, configBranch, currentBranch, configFile, change.From.Name, change.From.TreeEntry.Mode)
		}
		appendFolderToMap(changedFolderNamesMap, &changedConfigFile, configBranch, currentBranch, configFile, fileName, change.To.TreeEntry.Mode)
	}
	return maps.GetKeysFromMap(changedFolderNamesMap), changedConfigFile, nil
}

func getChangedFoldersOfCommitFiles(commit *object.Commit, configBranch string, currentBranch string, configFile string) ([]string, bool, error) {
	changedFolderNamesMap := make(map[string]bool)
	changedConfigFile := false
	fileIter, err := commit.Files()
	if err != nil {
		return nil, false, err
	}
	err = fileIter.ForEach(func(file *object.File) error {
		appendFolderToMap(changedFolderNamesMap, &changedConfigFile, configBranch, currentBranch, configFile, file.Name, file.Mode)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return maps.GetKeysFromMap(changedFolderNamesMap), changedConfigFile, nil
}

func getRepository(gitDir string) (*git.Repository, string, error) {
	log.Debug().Msgf("opened gitDir %s", gitDir)
	repository, err := git.PlainOpen(gitDir)
	if err != nil {
		return nil, "", err
	}
	currentBranch, err := getCurrentBranch(repository)
	if err != nil {
		return nil, "", err
	}
	return repository, currentBranch, nil
}

func getTargetCommitHash(triggeredFromWebhook bool, beforeCommitString, targetCommitString string) (*plumbing.Hash, error) {
	targetCommitHash := plumbing.NewHash(targetCommitString)
	if targetCommitHash == plumbing.ZeroHash {
		return nil, errors.New("invalid targetCommit")
	}
	if triggeredFromWebhook && strings.EqualFold(beforeCommitString, targetCommitString) {
		return nil, errors.New("beforeCommit cannot be equal to the targetCommit")
	}
	return &targetCommitHash, nil
}

func getCurrentBranch(repository *git.Repository) (string, error) {
	head, err := repository.Head()
	if err != nil {
		return "", err
	}
	branchHeadNamePrefix := "refs/heads/"
	branchHeadName := head.Name().String()
	if head.Name() == "HEAD" || !strings.HasPrefix(branchHeadName, branchHeadNamePrefix) {
		return "", errors.New("unexpected current git revision")
	}
	currentBranch := strings.TrimPrefix(branchHeadName, branchHeadNamePrefix)
	return currentBranch, nil
}

func appendFolderToMap(changedFolderNamesMap map[string]bool, changedConfigFile *bool, configBranch string, currentBranch string, configFile string, filePath string, fileMode filemode.FileMode) {
	if filePath == "" {
		return
	}
	folderName := ""
	if fileMode == filemode.Dir {
		folderName = filePath
	} else {
		folderName = filepath.Dir(filePath)
		if !*changedConfigFile && strings.EqualFold(configBranch, currentBranch) && strings.EqualFold(configFile, filePath) {
			*changedConfigFile = true
		}
		log.Debug().Msgf("- file: %s", filePath)
	}
	if _, ok := changedFolderNamesMap[folderName]; !ok {
		changedFolderNamesMap[folderName] = true
	}
}

func getBeforeCommitHash(commitHash string, repository *git.Repository) (*plumbing.Hash, error) {
	logIter, err := repository.Log(&git.LogOptions{
		Order: git.LogOrderBSF,
	})
	if err != nil {
		return nil, err
	}
	var hash plumbing.Hash
	err = logIter.ForEach(func(c *object.Commit) error {
		hash = c.Hash
		if len(commitHash) > 0 && c.Hash.String() == commitHash {
			return io.EOF
		}
		return nil
	})
	return &hash, err
}

func getBranchCommitHash(r *git.Repository, branchName string) (*plumbing.Hash, error) {
	// first, we try to resolve a local revision. If possible, this is best. This succeeds if code branch and config
	// branch are the same
	commitHash, err := r.ResolveRevision(plumbing.Revision(branchName))
	if err != nil {
		// on second try, we try to resolve the remote branch. This introduces a chance that the remote has been altered
		// with new hash after initial clone
		commitHash, err = r.ResolveRevision(plumbing.Revision(fmt.Sprintf("refs/remotes/origin/%s", branchName)))
		if err != nil {
			if strings.EqualFold(err.Error(), "reference not found") {
				return nil, fmt.Errorf("there is no branch %s or access to the repository", branchName)
			}
			return nil, err
		}
	}
	return commitHash, nil
}

// getGitCommitTags returns any git tags which point to commitHash
func getGitCommitTags(gitWorkspace string, commitHashString string) (string, error) {
	gitDir := getGitDir(gitWorkspace)
	r, err := git.PlainOpen(gitDir)
	if err != nil {
		return "", err
	}

	commitHash := plumbing.NewHash(commitHashString)

	log.Debug().Msgf("getting all tags for repository")
	tags, err := r.Tags()
	if err != nil {
		return "", err
	}
	var tagNames []string

	// List all tags, both lightweight tags and annotated tags and see if any tags point to HEAD reference.
	err = tags.ForEach(func(t *plumbing.Reference) error {
		log.Debug().Msgf("resolving commit hash of tag %s", t.Name())
		// using workaround to circumvent tag resolution bug documented at https://github.com/go-git/go-git/issues/204
		tagName := strings.TrimPrefix(string(t.Name()), "refs/tags/")
		tagRef, err := r.Tag(tagName)
		if err != nil {
			log.Warn().Msgf("could not resolve commit hash of tag %s: %v", t.Name(), err)
			return nil
		}
		revHash, err := r.ResolveRevision(plumbing.Revision(tagRef.Hash().String()))
		if err != nil {
			log.Warn().Msgf("could not resolve commit hash of tag %s: %v", t.Name(), err)
			return nil
		}
		if *revHash == commitHash {
			tagNames = append(tagNames, tagName)
		}
		return nil
	})
	if err != nil {
		log.Warn().Msgf("could not resolve tags: %v", err)
		return "", nil
	}

	tagNamesString := strings.Join(tagNames, " ")

	return tagNamesString, nil
}

// GetCommitHash returns commit hash from webhook commit ID that triggered job, if present. If not, returns HEAD of
// build branch
func GetCommitHash(gitWorkspace, webhookCommitId, branchName string) (string, error) {
	if webhookCommitId != "" {
		log.Debug().Msgf("got git commit hash %s from env var %s", webhookCommitId, defaults.RadixGithubWebhookCommitId)
		return webhookCommitId, nil
	}
	log.Debug().Msgf("determining git commit hash of HEAD of branch %s", branchName)
	gitCommitHash, err := GetCommitHashFromHead(gitWorkspace, branchName)
	log.Debug().Msgf("got git commit hash %s from HEAD of branch %s", gitCommitHash, branchName)
	return gitCommitHash, err
}

// GetChangesFromGitRepository Get changed folders in environments and if radixconfig.yaml was changed
func GetChangesFromGitRepository(gitWorkspace, radixConfigBranch, radixConfigFileName string, triggeredFromWebhook bool, targetCommitHash string, lastCommitHashesForEnvs commithash.EnvCommitHashMap) (map[string][]string, bool, error) {
	radixConfigWasChanged := false
	envChanges := make(map[string][]string)
	if len(lastCommitHashesForEnvs) == 0 {
		log.Info().Msgf("No changes in GitHub repository")
		return nil, false, nil
	}
	if strings.HasPrefix(radixConfigFileName, gitWorkspace) {
		radixConfigFileName = strings.TrimPrefix(strings.TrimPrefix(radixConfigFileName, gitWorkspace), "/")
	}
	log.Info().Msgf("Changes in GitHub repository:")
	for envName, radixDeploymentCommit := range lastCommitHashesForEnvs {
		changedFolders, radixConfigWasChangedInEnv, err := getGitAffectedResourcesBetweenCommits(gitWorkspace, radixConfigBranch, radixConfigFileName, triggeredFromWebhook, targetCommitHash, radixDeploymentCommit.CommitHash)
		envChanges[envName] = changedFolders
		if err != nil {
			return nil, false, err
		}
		radixConfigWasChanged = radixConfigWasChanged || radixConfigWasChangedInEnv
		printEnvironmentChangedFolders(envName, radixDeploymentCommit, targetCommitHash, changedFolders)
	}
	if radixConfigWasChanged {
		log.Info().Msgf("Radix config file was changed %s", radixConfigFileName)
	}
	return envChanges, radixConfigWasChanged, nil
}

func printEnvironmentChangedFolders(envName string, radixDeploymentCommit commithash.RadixDeploymentCommit, targetCommitHash string, changedFolders []string) {
	log.Info().Msgf("- for the environment %s", envName)
	if len(radixDeploymentCommit.RadixDeploymentName) == 0 {
		log.Info().Msgf(" from initial commit to commit %s:", targetCommitHash)
	} else {
		log.Info().Msgf(" after the commit %s (of the deployment %s) to the commit %s:", radixDeploymentCommit.CommitHash, radixDeploymentCommit.RadixDeploymentName, targetCommitHash)
	}
	sort.Strings(changedFolders)
	for _, folder := range changedFolders {
		log.Info().Msgf("  - %s", folder)
	}
}
