package git_test

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	tempGitWorkDir string
)

func init() {
	for i := 0; i < 10; i++ {
		tmpDir := path.Join("/", os.TempDir(), uuid.New().String())
		_, err := os.Stat(tmpDir)
		if err != nil {
			tempGitWorkDir = tmpDir
			return
		}
	}
	panic(errors.New("failed to create temporary workdir for git unzip tests"))
}

func unzip(archivePath string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(tempGitWorkDir, f.Name)
		fmt.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(tempGitWorkDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path")
		}
		if f.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			if err = os.MkdirAll(filePath, os.ModePerm); err != nil {
				return err
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

func getTestGitDir(testDataDir string) string {
	gitDirPath := path.Join(tempGitWorkDir, testDataDir)
	if _, err := os.Stat(gitDirPath); err != nil {
		panic(err)
	}
	return gitDirPath
}

func setupGitTest(testDataArchive, unzippedDir string) string {
	err := unzip(testDataArchive)
	if err != nil {
		panic(err)
	}
	return getTestGitDir(unzippedDir)
}

func tearDownGitTest() {
	err := os.RemoveAll(tempGitWorkDir)
	if err != nil {
		panic(err)
	}
}

func Test_DiffCommits(t *testing.T) {
	scenarios := map[string]struct {
		beforeCommit  string
		targetCommit  string
		expectedDiffs git.DiffEntries
		expectedError error
	}{
		"different commits": {
			targetCommit: "157014b59d6b24205b4fbf57165f0029c49d7963",
			beforeCommit: "0b9ee1f93639fff492c05b8d5e662301f508debe",
			expectedDiffs: git.DiffEntries{
				{Name: "app1/Dockerfile"},
				{Name: "app1/app.js"},
				{Name: "app2/Dockerfile"},
				{Name: "app2/app.js"},
			},
		},
		"different commits in reverse order should give same result": {
			targetCommit: "0b9ee1f93639fff492c05b8d5e662301f508debe",
			beforeCommit: "157014b59d6b24205b4fbf57165f0029c49d7963",
			expectedDiffs: git.DiffEntries{
				{Name: "app1/Dockerfile"},
				{Name: "app1/app.js"},
				{Name: "app2/Dockerfile"},
				{Name: "app2/app.js"},
			},
		},
		"different commits in different branches (before: dev, after: dev)": {
			targetCommit: "cb65b06a331b4e363f32c88a94d2fd145e38b2cf",
			beforeCommit: "dd81c903e402b4120cbf2431393ced321b243b45",
			expectedDiffs: git.DiffEntries{
				{Name: ".gitattributes"},
				{Name: ".gitignore"},
				{Name: "app1/app.js"},
				{Name: "app1/data/index.md"},
				{Name: "app1/data/level2/data.json"},
				{Name: "app1/data/level2/test-data-git-commits-blobless.zip"},
				{Name: "app1/data/level2/test_data.zip"},
				{Name: "app3/app.js"},
				{Name: "app3/data/index.md"},
				{Name: "app3/radixconfig.yaml"},
				{Name: "app4/app.js"},
				{Name: "radixconfig.yaml"},
			},
		},
		"same commit should give empty list": {
			targetCommit:  "fe148016c5fcd68e3a3b42d32f1c02586072defc",
			beforeCommit:  "fe148016c5fcd68e3a3b42d32f1c02586072defc",
			expectedDiffs: git.DiffEntries{},
		},
		"empty before commit includes changes in initial commit": {
			targetCommit: "936011be645e11a1a8773ca61686176dbf62ba81",
			beforeCommit: "",
			expectedDiffs: git.DiffEntries{
				{Name: "app1/Dockerfile"},
				{Name: "app1/app.js"},
				{Name: "radixconfig.yaml"},
				{Name: ".gitignore"},
			},
		},
		"invalid target commit": {
			targetCommit:  "invalid-value",
			beforeCommit:  "fe148016c5fcd68e3a3b42d32f1c02586072defc",
			expectedError: git.ErrCommitNotFound,
		},
		"invalid before commit": {
			targetCommit:  "fe148016c5fcd68e3a3b42d32f1c02586072defc",
			beforeCommit:  "invalid-value",
			expectedError: git.ErrCommitNotFound,
		},
	}

	zips := map[string]string{
		"test-data-git-commits-blobless.zip": "test-data-git-commits-blobless",
		"test-data-git-commits.zip":          "test-data-git-commits",
	}

	for zipFile, folder := range zips {
		gitWorkspacePath := setupGitTest(zipFile, folder)
		defer tearDownGitTest()

		for scenarioName, scenario := range scenarios {
			t.Run(fmt.Sprintf("%s (zip: %s, folder: %s)", scenarioName, zipFile, folder), func(t *testing.T) {
				repo, err := git.Open(gitWorkspacePath)
				require.NoError(t, err)
				diffs, err := repo.DiffCommits(scenario.beforeCommit, scenario.targetCommit)
				assert.ErrorIs(t, err, scenario.expectedError)

				if scenario.expectedError == nil {
					assert.ElementsMatch(t, scenario.expectedDiffs, diffs)
				}

			})
		}
	}
}

func Test_Checkout(t *testing.T) {
	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	defer tearDownGitTest()
	repo, err := git.Open(gitWorkspacePath)
	require.NoError(t, err)

	const (
		zipFile       = "/app1/data/level2/test-data-git-commits-blobless.zip"
		gitIgnoreFile = ".gitignore"
		featureFile   = "feature.MD"
		pngFile       = "/app1/data/level2/img1.png" // lfs file defined via glob in /app1/data/.gitattributes
	)

	type fileCheck struct {
		path  string
		exist bool
		size  int64
	}

	tests := map[string]struct {
		checkoutRef   string
		fileChecks    []fileCheck
		expectedError error
	}{
		"commit": {
			checkoutRef: "0b9ee1f93639fff492c05b8d5e662301f508debe",
			fileChecks: []fileCheck{
				{path: zipFile, exist: false},
				{path: gitIgnoreFile, exist: true, size: 5},
				{path: featureFile, exist: false},
				{path: pngFile, exist: false},
			},
		},
		"dev branch": {
			checkoutRef: "dev",
			fileChecks: []fileCheck{
				{path: zipFile, exist: false},
				{path: gitIgnoreFile, exist: true, size: 21},
				{path: featureFile, exist: false},
				{path: pngFile, exist: false},
			},
		},
		"main branch": {
			checkoutRef: "main",
			fileChecks: []fileCheck{
				{path: zipFile, exist: true, size: 131}, // Due to lfs
				{path: gitIgnoreFile, exist: true, size: 41},
				{path: featureFile, exist: false},
				{path: pngFile, exist: true, size: 129}, // Due to lfs
			},
		},
		"feature branch": {
			checkoutRef: "feature",
			fileChecks: []fileCheck{
				{path: zipFile, exist: true, size: 131}, // Due to lfs
				{path: gitIgnoreFile, exist: true, size: 41},
				{path: featureFile, exist: true, size: 27},
				{path: pngFile, exist: false},
			},
		},
		"branch with @(at) character in name": {
			checkoutRef: "feature3/@branch-with-at",
			fileChecks: []fileCheck{
				{path: zipFile, exist: true, size: 131}, // Due to lfs
				{path: gitIgnoreFile, exist: true, size: 41},
				{path: featureFile, exist: false},
				{path: pngFile, exist: true, size: 129}, // Due to lfs
			},
		},
		"non existing ref": {
			checkoutRef:   "non-existing",
			expectedError: git.ErrReferenceNotFound,
		},
	}

	for testName, testSpec := range tests {
		t.Run(testName, func(t *testing.T) {
			err := repo.Checkout(testSpec.checkoutRef)
			assert.ErrorIs(t, err, testSpec.expectedError)
			if testSpec.expectedError == nil {
				for _, fc := range testSpec.fileChecks {
					t.Run(fc.path, func(t *testing.T) {
						fileStat, err := os.Stat(path.Join(gitWorkspacePath, fc.path))
						if fc.exist {
							require.NoError(t, err)
							assert.Equal(t, fc.size, fileStat.Size())
						} else {
							assert.ErrorIs(t, err, os.ErrNotExist)
						}
					})
				}
			}
		})
	}
}

func Test_ResolveCommitForReference(t *testing.T) {
	tests := map[string]struct {
		reference      string
		expectedCommit string
		expectedError  error
	}{
		"get commit for branch main": {
			reference:      "main",
			expectedCommit: "3c572f685d2148b2f5a9776804a7d4b85f80f46c",
		},
		"get commit for branch dev": {
			reference:      "dev",
			expectedCommit: "b6804f12dc53029cfca29a4850d56ef7cda069e9",
		},
		"get commit for branch feature": {
			reference:      "feature",
			expectedCommit: "9e94330651540210772aaf4819c77ca7e1102b64",
		},
		"get commit for branch with @(at) character in name": {
			reference:      "feature3/@branch-with-at",
			expectedCommit: "88c4e14349eb98678abc9dc485c3fd6aecb1609b",
		},
		"get commit for lightweight tag v1": {
			reference:      "v1",
			expectedCommit: "b9d516dcccd38776a2c6a6cbadee9b876237e6a5",
		},
		"get commit for annotated tag v2": {
			reference:      "v2",
			expectedCommit: "512f70aaef2e35c8b2f7b3aed92e524b5890cd4d",
		},
		"get commit for non existing reference": {
			reference:     "non-existing",
			expectedError: git.ErrReferenceNotFound,
		},
	}

	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	defer tearDownGitTest()
	repo, err := git.Open(gitWorkspacePath)
	require.NoError(t, err)

	for testName, testSpec := range tests {
		t.Run(testName, func(t *testing.T) {
			commit, err := repo.ResolveCommitForReference(testSpec.reference)
			assert.ErrorIs(t, err, testSpec.expectedError)
			assert.Equal(t, testSpec.expectedCommit, commit)
		})
	}
}

func Test_ResolveTagsForCommit(t *testing.T) {
	tests := map[string]struct {
		commitHash   string
		expectedTags []string
	}{
		"commit with single lightweight tag": {
			commitHash:   "b9d516dcccd38776a2c6a6cbadee9b876237e6a5",
			expectedTags: []string{"v1"},
		},
		"commit with single annotated tag": {
			commitHash:   "512f70aaef2e35c8b2f7b3aed92e524b5890cd4d",
			expectedTags: []string{"v2"},
		},
		"commit with multiple lightweight tags": {
			commitHash:   "24baed7787ea319a10e387da1290242b91e34744",
			expectedTags: []string{"v3", "v4"},
		},
	}

	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	defer tearDownGitTest()
	repo, err := git.Open(gitWorkspacePath)
	require.NoError(t, err)

	for testName, testSpec := range tests {
		t.Run(testName, func(t *testing.T) {
			actualTags, err := repo.ResolveTagsForCommit(testSpec.commitHash) // not annotated
			require.NoError(t, err)
			assert.ElementsMatch(t, actualTags, testSpec.expectedTags)
		})
	}
}

func Test_IsAncestor(t *testing.T) {
	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	defer tearDownGitTest()
	repo, err := git.Open(gitWorkspacePath)
	require.NoError(t, err)

	tests := map[string]struct {
		ancestor           string
		other              string
		expectedIsAncestor bool
		expectedError      error
	}{
		"a commit reachable as parent for main branch: commit is ancestor of main branch": {
			ancestor:           "dd81c903e402b4120cbf2431393ced321b243b45",
			other:              "main",
			expectedIsAncestor: true,
		},
		"a commit reachable as parent for main branch: main branch is not ancestor of commit": {
			ancestor:           "main",
			other:              "dd81c903e402b4120cbf2431393ced321b243b45",
			expectedIsAncestor: false,
		},
		"a commit only reachable as parent feature branch: commit is not ancestor of main branch": {
			ancestor:           "9e94330651540210772aaf4819c77ca7e1102b64",
			other:              "main",
			expectedIsAncestor: false,
		},
		"a commit only reachable as parent feature branch: commit is ancestor of feature branch": {
			ancestor:           "9e94330651540210772aaf4819c77ca7e1102b64",
			other:              "feature",
			expectedIsAncestor: true,
		},
		"a tag for a commit reachable as parent for main branch: tag is ancestor of main branch": {
			ancestor:           "v1",
			other:              "main",
			expectedIsAncestor: true,
		},
		"a tag for a commit reachable as parent for main branch: main branch is not ancestor of tag": {
			ancestor:           "main",
			other:              "v1",
			expectedIsAncestor: false,
		},
		"a tag for a commit not reachable as parent for dev branch: tag is not ancestor of dev branch": {
			ancestor:           "v1",
			other:              "dev",
			expectedIsAncestor: false,
		},
		"a tag for a commit reachable as parent for feature branch: tag is ancestor of feature branch": {
			ancestor:           "v1",
			other:              "feature",
			expectedIsAncestor: true,
		},
		"a tag for a commit reachable as parent as tag for other commit: tag is ancestor of other tag": {
			ancestor:           "v1",
			other:              "v2",
			expectedIsAncestor: true,
		},
		"a tag for a commit not reachable as parent as tag for other commit: tag is not ancestor of other tag": {
			ancestor:           "v2",
			other:              "v1",
			expectedIsAncestor: false,
		},
		"a commit reachable as parent for v1 tag: commit is ancestor of v1 tag": {
			ancestor:           "085fc1ff420561f36464dea2445fc280fab66bca",
			other:              "v1",
			expectedIsAncestor: true,
		},
		"a commit not reachable (newer) as parent for v1 tag: commit is not ancestor of v1 tag": {
			ancestor:           "a23d865f9a06e6129b655937eb633ff6223d4867",
			other:              "v1",
			expectedIsAncestor: false,
		},
		"a commit reachable as parent for other commit: commit is ancestor of other commit": {
			ancestor:           "9a509b392ef5089246fd11efc0e66f8f4b931f88",
			other:              "dc35b641712d46b79371a9d531349e107b4f391e",
			expectedIsAncestor: true,
		},
		"a commit not reachable (newer) as parent for other commit: commit is not ancestor of other commit": {
			ancestor:           "dc35b641712d46b79371a9d531349e107b4f391e",
			other:              "9a509b392ef5089246fd11efc0e66f8f4b931f88",
			expectedIsAncestor: false,
		},
		"a branch is ancestor of itself": {
			ancestor:           "dev",
			other:              "dev",
			expectedIsAncestor: true,
		},
		"a tag is ancestor of itself": {
			ancestor:           "v2",
			other:              "v2",
			expectedIsAncestor: true,
		},
		"a commit is ancestor of itself": {
			ancestor:           "cfe04e403e57ce1d803519b826135d64d1c066d0",
			other:              "cfe04e403e57ce1d803519b826135d64d1c066d0",
			expectedIsAncestor: true,
		},
		"the head commit of a branch is ancestor to the branch": {
			ancestor:           "9e94330651540210772aaf4819c77ca7e1102b64",
			other:              "feature",
			expectedIsAncestor: true,
		},
		"the commit of a tag is ancestor to the tag": {
			ancestor:           "24baed7787ea319a10e387da1290242b91e34744",
			other:              "v3",
			expectedIsAncestor: true,
		},
		"allow branch with @(at) character in name": {
			ancestor:           "feature3/@branch-with-at",
			other:              "feature3/@branch-with-at",
			expectedIsAncestor: true,
		},
		"non-existing ancestor returns error": {
			ancestor:      "non-existing",
			other:         "main",
			expectedError: git.ErrCommitNotFound,
		},
		"non-existing other returns error": {
			ancestor:      "main",
			other:         "non-existing",
			expectedError: git.ErrCommitNotFound,
		},
	}

	for testName, testSpec := range tests {
		t.Run(testName, func(t *testing.T) {
			isAncestor, err := repo.IsAncestor(testSpec.ancestor, testSpec.other)
			assert.ErrorIs(t, err, testSpec.expectedError)
			assert.Equal(t, testSpec.expectedIsAncestor, isAncestor)
		})
	}
}
