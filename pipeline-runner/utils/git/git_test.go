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
		beforeCommitExclusive  string
		targetCommit           string
		expectedChangedFolders []string
		expectedError          error
	}{
		"init - add radixconfig and gitignore files": {
			targetCommit:           "7d6309f7537baa2815bb631802e6d8d613150c52",
			beforeCommitExclusive:  "",
			expectedChangedFolders: []string{"."},
		},
		"init - add radixconfig and gitignore files with same commit": {
			targetCommit:           "7d6309f7537baa2815bb631802e6d8d613150c52",
			beforeCommitExclusive:  "7d6309f7537baa2815bb631802e6d8d613150c52",
			expectedChangedFolders: nil,
		},
		"added app1 folder and its files. app1 component added to the radixconfig": {
			targetCommit:           "0b9ee1f93639fff492c05b8d5e662301f508debe",
			beforeCommitExclusive:  "7d6309f7537baa2815bb631802e6d8d613150c52",
			expectedChangedFolders: []string{".", "app1"},
		},
		"Changed radixconfig, but config branch is different": {
			targetCommit:           "0b9ee1f93639fff492c05b8d5e662301f508debe",
			beforeCommitExclusive:  "7d6309f7537baa2815bb631802e6d8d613150c52",
			expectedChangedFolders: []string{".", "app1"},
		},
		"changed files in the folder app1": {
			targetCommit:           "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			beforeCommitExclusive:  "0b9ee1f93639fff492c05b8d5e662301f508debe",
			expectedChangedFolders: []string{"app1"},
		},
		"the same target and before commit, files were changed": {
			targetCommit:           "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			beforeCommitExclusive:  "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			expectedChangedFolders: nil,
		},
		"the same target and before commit, only radixconfig was changed": {
			targetCommit:           "38845f7ba0b9dbfa0a0a929aaddc7308f4db35e2",
			beforeCommitExclusive:  "38845f7ba0b9dbfa0a0a929aaddc7308f4db35e2",
			expectedChangedFolders: nil,
		},
		"invalid target commit": {
			targetCommit:          "invalid-commit",
			beforeCommitExclusive: "",
			expectedError:         git.ErrCommitNotFound,
		},
		"invalid empty target commit": {
			targetCommit:          "",
			beforeCommitExclusive: "",
			expectedError:         git.ErrEmptyCommitHash,
		},
		"Added folder app2 with files": {
			targetCommit:           "157014b59d6b24205b4fbf57165f0029c49d7963",
			beforeCommitExclusive:  "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			expectedChangedFolders: []string{"app2"},
		},
		"Folder app2 was renamed to app3, file in folder 3 was changed": {
			targetCommit:           "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:  "157014b59d6b24205b4fbf57165f0029c49d7963",
			expectedChangedFolders: []string{"app2", "app3"},
		},
		"File in folder 3 was changed": {
			targetCommit:           "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:  "8d99f5318a7164ff1e03bd4fbe1cba554e9c906b",
			expectedChangedFolders: []string{"app3"},
		},
		"radixconfig.yaml was changed": {
			targetCommit:           "38845f7ba0b9dbfa0a0a929aaddc7308f4db35e2",
			beforeCommitExclusive:  "2127927fa21ae471baefbadd3f05b60a4bf38b5f",
			expectedChangedFolders: []string{"."},
		},
		"File in folder 3 was changed, same commit": {
			targetCommit:           "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:  "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			expectedChangedFolders: nil,
		},
		"radixconfig.yaml was added to the folder app3": {
			targetCommit:           "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			beforeCommitExclusive:  "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			expectedChangedFolders: []string{"app3"},
		},
		"radixconfig.yaml was added to the folder app3, it is current config": {
			targetCommit:           "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			beforeCommitExclusive:  "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			expectedChangedFolders: []string{"app3"},
		},
		"radixconfig.yaml was changed to the folder app3": {
			targetCommit:           "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			beforeCommitExclusive:  "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			expectedChangedFolders: []string{"app3"},
		},
		"radixconfig.yaml was changed to the folder app3, it is current config": {
			targetCommit:           "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			beforeCommitExclusive:  "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			expectedChangedFolders: []string{"app3"},
		},
		"Sub-folders were added to the folder app3, with file. Sub-folder was added to the folder app1, without file.": {
			targetCommit:           "13bd8316267d6a0be44f3f87fe49e807e7bc24b4",
			beforeCommitExclusive:  "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			expectedChangedFolders: []string{"app3/data/level2"},
		},
		"Files added and changed in the sub-folders of app3. File was added to the sub-folder in the folder app1.": {
			targetCommit:           "986065b74c8e9e4012287fdd6b13021591ce00c3",
			beforeCommitExclusive:  "13bd8316267d6a0be44f3f87fe49e807e7bc24b4",
			expectedChangedFolders: []string{"app3", "app3/data", "app3/data/level2", "app1/data"},
		},
		"radixconfig-app3-level2.yaml was added to the subfolder of the folder app3": {
			targetCommit:           "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:  "986065b74c8e9e4012287fdd6b13021591ce00c3",
			expectedChangedFolders: []string{"app3/data/level2"},
		},
		"radixconfig-app3-level2.yaml was added to the subfolder of the folder app3, it is current config": {
			targetCommit:           "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:  "986065b74c8e9e4012287fdd6b13021591ce00c3",
			expectedChangedFolders: []string{"app3/data/level2"},
		},
		"radixconfig-app3-level2.yaml was added to the subfolder of the folder app3, it is current config, but not this branch": {
			targetCommit:           "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:  "986065b74c8e9e4012287fdd6b13021591ce00c3",
			expectedChangedFolders: []string{"app3/data/level2"},
		},
		"Files were changed in subfolders app1 and app3, with config radixconfig.yaml": {
			targetCommit:           "2127927fa21ae471baefbadd3f05b60a4bf38b5f",
			beforeCommitExclusive:  "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			expectedChangedFolders: []string{"app1/data", "app3/data/level2"},
		},
		"Files were changed in subfolders app1 and app3, with config app3/data/level2/radixconfig-app3-level2.yaml": {
			targetCommit:           "2127927fa21ae471baefbadd3f05b60a4bf38b5f",
			beforeCommitExclusive:  "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			expectedChangedFolders: []string{"app1/data", "app3/data/level2"},
		},
		"Empty commit, made with 'git commit -m 'empty' --allow-empty": {
			targetCommit:           "95b2136dd7f8bd8ec52ef46897d55ba321da3abe",
			beforeCommitExclusive:  "cdbd98a036ae5a96f5ad97d2fb225aa5e550f4c1",
			expectedChangedFolders: []string{},
		},
		"Changes of in-root file, used in the component with root source": {
			targetCommit:           "8b5f81bd11c1670a6894cd14abcb1c9d2cde6e7e",
			beforeCommitExclusive:  "d897c65f4322f4dc71b33ff2fd6b365f815a2026",
			expectedChangedFolders: []string{"."},
		},
		"Changes of subfolder, used in the component with root source": {
			targetCommit:           "192e173dc968368a5dc9f42abbcbd9dab6484194",
			beforeCommitExclusive:  "8b5f81bd11c1670a6894cd14abcb1c9d2cde6e7e",
			expectedChangedFolders: []string{"app3/data/level2"},
		},
		"Change of binary files is tracked correctly": {
			targetCommit:           "efa70bd57f3a965aa13429ca856f54feaf3d2645",
			beforeCommitExclusive:  "4a7624fd54c19adbf72bba72b0ae16829680db34",
			expectedChangedFolders: []string{"app1/data/level2"},
		},
		"Change of LFS binary files is tracked correctly": {
			targetCommit:           "cd65c2fcab588953c72f0af5350282c282051286",
			beforeCommitExclusive:  "dd81c903e402b4120cbf2431393ced321b243b45",
			expectedChangedFolders: []string{"app1/data/level2"},
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
				diffs, err := repo.DiffCommits(scenario.beforeCommitExclusive, scenario.targetCommit)
				if scenario.expectedError == nil {
					assert.NoError(t, err)
					assert.ElementsMatch(t, scenario.expectedChangedFolders, diffs.Dirs())
				} else {
					assert.ErrorIs(t, err, scenario.expectedError)
				}
			})
		}
	}
}

func Test_Checkout(t *testing.T) {
	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	defer tearDownGitTest()

	var (
		zipFile     string = path.Join(gitWorkspacePath, "/app1/data/level2/test-data-git-commits-blobless.zip")
		dockerFile  string = path.Join(gitWorkspacePath, "app1/Dockerfile")
		featureFile string = path.Join(gitWorkspacePath, "feature.MD")
	)

	repo, err := git.Open(gitWorkspacePath)
	require.NoError(t, err)

	err = repo.Checkout("0b9ee1f93639fff492c05b8d5e662301f508debe")
	assert.NoError(t, err)
	_, err = os.Stat(zipFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(featureFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
	fstat, err := os.Stat(dockerFile)
	require.NoError(t, err)
	assert.Equal(t, int64(84), fstat.Size())

	err = repo.Checkout("dev")
	assert.NoError(t, err)
	_, err = os.Stat(zipFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(featureFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
	fstat, err = os.Stat(dockerFile)
	require.NoError(t, err)
	assert.Equal(t, int64(88), fstat.Size())

	err = repo.Checkout("main")
	assert.NoError(t, err)
	_, err = os.Stat(zipFile)
	assert.NoError(t, err)
	_, err = os.Stat(featureFile)
	assert.ErrorIs(t, err, os.ErrNotExist)
	fstat, err = os.Stat(dockerFile)
	require.NoError(t, err)
	assert.Equal(t, int64(88), fstat.Size())

	err = repo.Checkout("feature")
	assert.NoError(t, err)
	_, err = os.Stat(zipFile)
	assert.NoError(t, err)
	_, err = os.Stat(featureFile)
	assert.NoError(t, err)
	fstat, err = os.Stat(dockerFile)
	require.NoError(t, err)
	assert.Equal(t, int64(88), fstat.Size())

	err = repo.Checkout("non-existing")
	assert.Error(t, err)
}

func Test_ResolveCommitForReference(t *testing.T) {
	tests := map[string]struct {
		reference      string
		expectedCommit string
		expectedError  error
	}{
		"get commit for branch main": {
			reference:      "main",
			expectedCommit: "cd65c2fcab588953c72f0af5350282c282051286",
		},
		"get commit for branch dev": {
			reference:      "dev",
			expectedCommit: "b6804f12dc53029cfca29a4850d56ef7cda069e9",
		},
		"get commit for branch feature": {
			reference:      "feature",
			expectedCommit: "9e94330651540210772aaf4819c77ca7e1102b64",
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
