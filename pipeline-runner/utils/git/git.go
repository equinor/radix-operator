package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pipeline-runner/utils/radix/deployment/commithash"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/rs/zerolog/log"
)

type Repository interface {
	CheckoutBranch(branch string) error
	CheckoutCommit(commit string) error
	GetLatestCommitForBranch(branch string) (string, error)
	IsAncestor(ancestor, other string) (bool, error)
	ResolveTagsForCommit(commit string) ([]string, error)
}

func Open(path string) (Repository, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	return &repository{repo: r}, nil
}

type repository struct {
	repo *git.Repository
}

func (r *repository) CheckoutBranch(branch string) error {
	return r.checkoutBranchOrCommit(branch, "")
}

func (r *repository) CheckoutCommit(commit string) error {
	return r.checkoutBranchOrCommit("", commit)
}

func (r *repository) GetLatestCommitForBranch(branch string) (string, error) {
	if _, err := r.repo.Branch(branch); err != nil {
		return "", err
	}

	commitHash, err := r.repo.ResolveRevision(plumbing.Revision(branch))
	if err != nil {
		return "", err
	}
	return commitHash.String(), nil
}

func (r *repository) IsAncestor(ancestor, other string) (bool, error) {
	ancestorHash, err := r.repo.ResolveRevision(plumbing.Revision(ancestor))
	if err != nil {
		return false, err
	}

	otherHash, err := r.repo.ResolveRevision(plumbing.Revision(other))
	if err != nil {
		return false, err
	}

	ancestorCommit, err := r.repo.CommitObject(*ancestorHash)
	if err != nil {
		return false, err
	}

	otherCommit, err := r.repo.CommitObject(*otherHash)
	if err != nil {
		return false, err
	}

	return ancestorCommit.IsAncestor(otherCommit)
}

func (r *repository) ResolveTagsForCommit(commit string) ([]string, error) {
	commitHash := plumbing.NewHash(commit)

	tags, err := r.repo.Tags()
	if err != nil {
		return nil, err
	}
	var tagNames []string

	// List all tags, both lightweight tags and annotated tags and see if any tags point to HEAD reference.
	err = tags.ForEach(func(t *plumbing.Reference) error {
		// using workaround to circumvent tag resolution bug documented at https://github.com/go-git/go-git/issues/204
		tagName := strings.TrimPrefix(string(t.Name()), "refs/tags/")
		tagRef, err := r.repo.Tag(tagName)
		if err != nil {
			return err
		}
		revHash, err := r.repo.ResolveRevision(plumbing.Revision(tagRef.Hash().String()))
		if err != nil {
			return err
		}
		if *revHash == commitHash {
			tagNames = append(tagNames, tagName)
		}
		return nil
	})

	return tagNames, err
}

// Performs git checkout of a branch or commit.
// branch and commit are mutually exclusive
func (r *repository) checkoutBranchOrCommit(branch, commit string) error {
	if len(branch) > 0 && len(commit) > 0 {
		return errors.New("branch and commit are mutually exclusive")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if len(commit) > 0 {
		return worktree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)})
	}

	return worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch)})
}

func getGitDir(gitWorkspace string) string {
	return gitWorkspace + "/.git"
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
