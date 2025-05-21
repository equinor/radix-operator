package git

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

const (
	remote = "origin"
)

var (
	ErrReferenceNotFound = errors.New("reference not found")
	ErrCommitNotFound    = errors.New("commit not found")
)

type Repository interface {
	// Checkout a specific commit, or the commit of a branch or tag name.
	Checkout(reference string) error
	// Get the current commit for a branch or tag name.
	GetCommitForReference(reference string) (string, error)
	// Checks if a commit, branch or tag is ancestor of other commit, branch or tag.
	IsAncestor(ancestor, other string) (bool, error)
	// Returns tags for a specific commit.
	ResolveTagsForCommit(commitHash string) ([]string, error)
	// Returns list of changes between commits.
	// If beforeCommitHash is empty, all changes up till targetCommit is returned.
	DiffCommits(beforeCommitHash, afterCommitHash string) (DiffEntries, error)
}

type DiffEntry struct {
	Name  string
	IsDir bool
}

type DiffEntries []DiffEntry

// Returns list of distinct directories
func (s DiffEntries) Dirs() []string {
	allDirs := slice.Map(s, func(e DiffEntry) string {
		if e.IsDir {
			return e.Name
		}
		return path.Dir(e.Name)
	})
	slices.Sort(allDirs)
	return slices.Compact(allDirs)
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

	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return ErrReferenceNotFound
		}
		return fmt.Errorf("failed to checkout: %w", err)
	}

	return nil
}

func (r *repository) resolveHashForReference(reference string) (plumbing.Hash, bool, error) {
	tryRefs := []plumbing.ReferenceName{
		plumbing.NewTagReferenceName(reference),
		plumbing.NewBranchReferenceName(reference),
		plumbing.NewRemoteReferenceName(remote, reference),
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

func (r *repository) GetCommitForReference(reference string) (string, error) {
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
		return false, fmt.Errorf("failed to resolve ancestor: %w", err)
	}
	if !found {
		ancestorHash = plumbing.NewHash(ancestor)
	}

	otherHash, found, err := r.resolveHashForReference(other)
	if err != nil {
		return false, fmt.Errorf("failed to resolve other: %w", err)
	}
	if !found {
		otherHash = plumbing.NewHash(ancestor)
	}

	ancestorCommit, err := r.repo.CommitObject(ancestorHash)
	if err != nil {
		return false, err
	}

	otherCommit, err := r.repo.CommitObject(otherHash)
	if err != nil {
		return false, err
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

func (r *repository) DiffCommits(beforeCommitHash, afterCommitHash string) (DiffEntries, error) {
	var (
		beforeCommit, afterCommit *object.Commit
		beforeTree, afterTree     *object.Tree
		err                       error
	)

	if afterCommit, err = r.repo.CommitObject(plumbing.NewHash(afterCommitHash)); err != nil {
		return nil, err
	}

	if afterTree, err = afterCommit.Tree(); err != nil {
		return nil, err
	}

	if len(beforeCommitHash) > 0 {
		if beforeCommit, err = r.repo.CommitObject(plumbing.NewHash(beforeCommitHash)); err != nil {
			return nil, err
		}

		if beforeTree, err = beforeCommit.Tree(); err != nil {
			return nil, err
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

func newDiffEntry(c object.ChangeEntry) DiffEntry {
	return DiffEntry{
		Name:  c.Name,
		IsDir: c.TreeEntry.Mode == filemode.Dir,
	}
}
