package git

import (
	"context"
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
	if err = worktree.Reset(&git.ResetOptions{
		Commit: commitHash,
		Mode:   git.HardReset,
	}); err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return fmt.Errorf("commit %s not found", commitHashString)
		}
		return fmt.Errorf("unable to reset git head %s: %w", commitHashString, err)
	}
	log.Debug().Msgf("reset HEAD to %s", commitHashString)
	return nil
}

// GetCommitHashAndTags gets target commit hash and tags from GitHub repository
func GetCommitHashAndTags(gitWorkspace, commitId, branchName, gitEventRefsType string) (string, string, error) {
	targetCommitHash, err := GetCommitHash(gitWorkspace, commitId, branchName, gitEventRefsType)
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
func GetCommitHashFromHead(gitWorkspace string, branchName string, gitEventRefsType string) (string, error) {
	gitDir := getGitDir(gitWorkspace)
	r, err := git.PlainOpen(gitDir)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("opened gitDir %s", gitDir)

	// Get branchName hash
	commitHash, err := getBranchCommitHash(r, branchName, gitEventRefsType)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("resolved s% %s", gitEventRefsType, branchName)

	hashBytesString := hex.EncodeToString(commitHash[:])
	return hashBytesString, nil
}

// getGitAffectedResourcesBetweenCommits returns the list of folders, where files were affected after beforeCommitHash (not included) till targetCommitHash commit (included)
func getGitAffectedResourcesBetweenCommits(gitWorkspace, configBranch, configFile, targetCommitString, beforeCommitString string) ([]string, bool, error) {
	if len(targetCommitString) == 0 {
		return nil, false, fmt.Errorf("invalid empty targetCommit")
	}
	if strings.EqualFold(targetCommitString, beforeCommitString) { // same commit, no source changes
		return nil, false, nil
	}
	gitDir := getGitDir(gitWorkspace)
	repository, currentBranch, err := getRepository(gitDir)
	if err != nil {
		return nil, false, err
	}
	targetCommit, err := findCommit(targetCommitString, repository)
	if err != nil {
		return nil, false, err
	}
	if targetCommit == nil {
		return nil, false, errors.New("invalid targetCommit")
	}

	beforeCommit, err := findCommit(beforeCommitString, repository)
	if err != nil {
		return nil, false, err
	}

	return getChangedFoldersFromTargetCommitTillExclusiveBeforeCommit(targetCommit, beforeCommit, configBranch, currentBranch, configFile)
}

func getChangedFoldersFromTargetCommitTillExclusiveBeforeCommit(targetCommit *object.Commit, beforeCommit *object.Commit, configBranch string, currentBranch string, configFile string) ([]string, bool, error) {
	if targetCommit == nil {
		return nil, false, errors.New("targetCommit must be set")
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		return nil, false, err
	}

	var beforeTree *object.Tree
	if beforeCommit != nil {
		beforeTree, err = beforeCommit.Tree()
		if err != nil {
			return nil, false, err
		}
	}
	changes, err := object.DiffTreeContext(context.TODO(), beforeTree, targetTree)
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

// findCommit will return a Hash if found, or nil if not found.
func findCommit(commitHash string, repository *git.Repository) (*object.Commit, error) {
	logIter, err := repository.Log(&git.LogOptions{
		Order: git.LogOrderBSF, // sorted from latest down to oldest
	})
	if err != nil {
		return nil, err
	}

	var hash plumbing.Hash
	err = logIter.ForEach(func(c *object.Commit) error {
		if c.Hash.String() == commitHash {
			hash = c.Hash
			return io.EOF
		}
		return nil // continue iteration loop
	})

	if err != io.EOF {
		return nil, err
	}

	if hash.IsZero() {
		return nil, nil
	}

	return repository.CommitObject(hash)
}

func getBranchCommitHash(r *git.Repository, branchName, gitEventRefsType string) (*plumbing.Hash, error) {
	// first, we try to resolve a local revision. If possible, this is best. This succeeds if code branch and config
	// branch are the same
	commitHash, err := r.ResolveRevision(plumbing.Revision(branchName))
	if err != nil {
		// on second try, we try to resolve the remote branch. This introduces a chance that the remote has been altered
		// with new hash after initial clone
		commitHash, err = r.ResolveRevision(plumbing.Revision(fmt.Sprintf("refs/remotes/origin/%s", branchName)))
		if err != nil {
			if strings.EqualFold(err.Error(), "reference not found") {
				return nil, fmt.Errorf("there is no s% %s or access to the repository", gitEventRefsType, branchName)
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
func GetCommitHash(gitWorkspace, commitId, branchName, gitEventRefsType string) (string, error) {
	if commitId != "" {
		log.Debug().Msgf("got git commit hash %s from env var %s", commitId, defaults.RadixCommitIdEnvironmentVariable)
		return commitId, nil
	}
	log.Debug().Msgf("determining git commit hash of HEAD of s% %s", gitEventRefsType, branchName)
	gitCommitHash, err := GetCommitHashFromHead(gitWorkspace, branchName, gitEventRefsType)
	log.Debug().Msgf("got git commit hash %s from HEAD of s% %s", gitCommitHash, gitEventRefsType, branchName)
	return gitCommitHash, err
}

// GetChangesFromGitRepository Get changed folders in environments and if radixconfig.yaml was changed
func GetChangesFromGitRepository(gitWorkspace, radixConfigBranch, radixConfigFileName, targetCommitHash string, lastCommitHashesForEnvs commithash.EnvCommitHashMap) (map[string][]string, bool, error) {
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
		changedFolders, radixConfigWasChangedInEnv, err := getGitAffectedResourcesBetweenCommits(gitWorkspace, radixConfigBranch, radixConfigFileName, targetCommitHash, radixDeploymentCommit.CommitHash)
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
