package git

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/gitattributes"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"golang.org/x/exp/maps"
)

var (
	ErrReferenceNotFound = errors.New("reference not found")
	ErrCommitNotFound    = errors.New("commit not found")
	ErrEmptyCommitHash   = errors.New("empty commit hash")
	ErrUnstagedChanges   = errors.New("worktree contains unstaged changes")
)

type Repository interface {
	// Checkout a specific commit, or the commit of a branch or tag name.
	Checkout(reference string) error
	// Get the current commit for a branch or tag name.
	ResolveCommitForReference(reference string) (string, error)
	// Checks if a commit, branch or tag is ancestor of other commit, branch or tag.
	IsAncestor(ancestor, other string) (bool, error)
	// Returns tags for a specific commit.
	ResolveTagsForCommit(commitHash string) ([]string, error)
	// Returns list of changes between commits.
	// If beforeCommitHash is empty, all changes up till targetCommit is returned.
	DiffCommits(beforeCommitHash, targetCommitHash string) (DiffEntries, error)
}

type DiffEntry struct {
	Name  string
	IsDir bool
}

type DiffEntries []DiffEntry

func Open(path string) (Repository, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	remoteObjs, err := r.Remotes()
	if err != nil {
		return nil, err
	}
	remotes := slice.Map(remoteObjs, func(o *git.Remote) string { return o.Config().Name })

	return &repository{repo: r, remotes: remotes}, nil
}

type repository struct {
	repo    *git.Repository
	remotes []string
}

func (r *repository) Checkout(reference string) error {
	hash, found, err := r.resolveHashForReference(reference)
	if err != nil {
		return err
	}
	if !found {
		hash = plumbing.NewHash(reference)
	}

	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	// HACK: go-git does not support lfs files. If the repo has lfs files, go-git will report them as non clean, and fail when calling Checkout.
	// We will thereforce perform a hard reset if all files reported as changed are in the lisrt of lfs files in .gitattributes
	// TODO: Check if these steps are neccessary when go-git v6 is released.
	if status, err := wt.Status(); err != nil {
		return nil
	} else if !status.IsClean() {
		lfsFiles, err := r.lfsFiles(wt)
		if err != nil {
			return nil
		}

		dirtyFiles := maps.Keys(status)
		safeToHardReset := slice.All(dirtyFiles, func(f string) bool {
			return slices.Contains(lfsFiles, f)
		})

		if safeToHardReset {
			if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
				return err
			}
		}
	}

	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		switch {
		case errors.Is(err, plumbing.ErrReferenceNotFound):
			return ErrReferenceNotFound
		case errors.Is(err, git.ErrUnstagedChanges):
			return ErrUnstagedChanges
		default:
			return err
		}
	}

	return nil
}

func (r *repository) ResolveCommitForReference(reference string) (string, error) {
	hash, found, err := r.resolveHashForReference(reference)
	if err != nil {
		return "", fmt.Errorf("failed to resolve reference: %w", err)
	}
	if !found {
		return "", ErrReferenceNotFound
	}
	return hash.String(), nil
}

func (r *repository) IsAncestor(ancestor, other string) (bool, error) {
	ancestorHash, found, err := r.resolveHashForReference(ancestor)
	if err != nil {
		return false, fmt.Errorf("failed to resolve ancestor reference: %w", err)
	} else if found {
		ancestor = ancestorHash.String()
	}

	otherHash, found, err := r.resolveHashForReference(other)
	if err != nil {
		return false, fmt.Errorf("failed to resolve other reference: %w", err)
	} else if found {
		other = otherHash.String()
	}

	ancestorCommit, err := r.resolveCommitFromHash(ancestor)
	if err != nil {
		return false, fmt.Errorf("failed to resolve ancestor hash: %w", err)
	}

	otherCommit, err := r.resolveCommitFromHash(other)
	if err != nil {
		return false, fmt.Errorf("failed to resolve other hash: %w", err)
	}

	return ancestorCommit.IsAncestor(otherCommit)
}

func (r *repository) ResolveTagsForCommit(commitHash string) ([]string, error) {
	hash := plumbing.NewHash(commitHash)

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
		if *revHash == hash {
			tagNames = append(tagNames, tagName)
		}
		return nil
	})

	return tagNames, err
}

func (r *repository) DiffCommits(beforeCommitHash, targetCommitHash string) (DiffEntries, error) {
	var (
		beforeTree, afterTree *object.Tree
		err                   error
	)

	if afterTree, err = r.resolveTreeFromCommitHash(targetCommitHash); err != nil {
		return nil, fmt.Errorf("failed to resolve target commit: %w", err)
	}

	if len(beforeCommitHash) > 0 {
		if beforeTree, err = r.resolveTreeFromCommitHash(beforeCommitHash); err != nil {
			return nil, fmt.Errorf("failed to resolve before commit: %w", err)
		}
	}

	changes, err := object.DiffTree(beforeTree, afterTree)
	if err != nil {
		return nil, err
	}

	changedFiles := make(DiffEntries, 0, len(changes))
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, err
		}

		switch action {
		case merkletrie.Insert, merkletrie.Modify:
			changedFiles = append(changedFiles, newDiffEntry(change.To))
		default:
			changedFiles = append(changedFiles, newDiffEntry(change.From))
		}
	}

	return changedFiles, nil
}

func (r *repository) lfsFiles(wt *git.Worktree) ([]string, error) {
	attrs, err := gitattributes.ReadPatterns(wt.Filesystem, nil)
	if err != nil {
		return nil, err
	}

	lfsAttrs := slice.FindAll(attrs, func(ma gitattributes.MatchAttribute) bool {
		return slice.Any(ma.Attributes, func(a gitattributes.Attribute) bool { return a.Value() == "lfs" })
	})

	return slice.Map(lfsAttrs, func(ma gitattributes.MatchAttribute) string { return ma.Name }), nil
}

func (r *repository) resolveHashForReference(reference string) (plumbing.Hash, bool, error) {
	tryRefs := []plumbing.ReferenceName{
		plumbing.NewTagReferenceName(reference),
		plumbing.NewBranchReferenceName(reference),
	}

	for _, remote := range r.remotes {
		tryRefs = append(tryRefs, plumbing.NewRemoteReferenceName(remote, reference))
	}

	for _, ref := range tryRefs {
		if hash, err := r.repo.ResolveRevision(plumbing.Revision(ref)); err == nil {
			return *hash, true, nil
		} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return plumbing.Hash{}, false, err
		}
	}

	return plumbing.Hash{}, false, nil
}

func (r *repository) resolveTreeFromCommitHash(commitHash string) (*object.Tree, error) {
	if len(commitHash) == 0 {
		return nil, ErrEmptyCommitHash
	}

	commit, err := r.resolveCommitFromHash(commitHash)
	if err != nil {
		return nil, err
	}

	return commit.Tree()
}

func (r *repository) resolveCommitFromHash(commitHash string) (*object.Commit, error) {
	commit, err := r.repo.CommitObject(plumbing.NewHash(commitHash))
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, ErrCommitNotFound
	}
	return commit, err
}

func newDiffEntry(c object.ChangeEntry) DiffEntry {
	return DiffEntry{
		Name:  c.Name,
		IsDir: c.TreeEntry.Mode == filemode.Dir,
	}
}
