package git

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/utils/logger"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unzipDestination = "7c55c884-7a3e-4b1d-bb03-e7f8ce235d50"

func unzip(archivePath string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		panic(err)
	}
	defer archive.Close()
	_, err = os.Stat(unzipDestination)
	if err == nil {
		err := os.RemoveAll(unzipDestination)
		if err != nil {
			return err
		}
	}
	for _, f := range archive.File {
		filePath := filepath.Join(unzipDestination, f.Name)
		fmt.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(unzipDestination)+string(os.PathSeparator)) {
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
	workingDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	gitDirPath := fmt.Sprintf("%s/%s/%s", workingDir, unzipDestination, testDataDir)
	_, err = os.Stat(gitDirPath)
	if err != nil {
		panic(err)
	}
	return gitDirPath
}

func TestGetGitCommitHashFromHead_DummyRepo(t *testing.T) {
	gitDirPath := setupGitTest("test_data.zip", "test_data")

	releaseBranchHeadCommitHash := "43332ef8f8a8c3830a235a5af7ac9098142e3af8"
	commitHash, err := GetCommitHashFromHead(gitDirPath, "release")
	assert.NoError(t, err)
	assert.Equal(t, commitHash, releaseBranchHeadCommitHash)

	tearDownGitTest()
}

func setupGitTest(testDataArchive, unzippedDir string) string {
	err := unzip(testDataArchive)
	if err != nil {
		panic(err)
	}
	return getTestGitDir(unzippedDir)
}

func tearDownGitTest() {
	err := os.RemoveAll(unzipDestination)
	if err != nil {
		panic(err)
	}
}

func TestGetGitCommitHashFromHead_DummyRepo2(t *testing.T) {
	setupLog()
	gitDirPath := setupGitTest("test_data2.zip", "test_data2")

	releaseBranchHeadCommitHash := "a1ee44808de2a42d291b59fefb5c66b8ff6bf898"
	commitHash, err := GetCommitHashFromHead(gitDirPath, "this-branch-is-only-remote")
	assert.NoError(t, err)
	assert.Equal(t, commitHash, releaseBranchHeadCommitHash)

	tearDownGitTest()
}

func TestGetGitCommitTags(t *testing.T) {
	setupLog()
	gitDirPath := setupGitTest("test_data.zip", "test_data")

	branchName := "branch-with-tags"
	tag0 := "special&%¤tag"
	tag1 := "tag-contains@at-sign"
	tag2 := "v1.12"

	commitHash, err := GetCommitHashFromHead(gitDirPath, branchName)
	assert.NoError(t, err)
	tagsString, err := getGitCommitTags(gitDirPath, commitHash)
	assert.NoError(t, err)
	tags := strings.Split(tagsString, " ")
	assert.Equal(t, tag0, tags[0])
	assert.Equal(t, tag1, tags[1])
	assert.Equal(t, tag2, tags[2])

	tearDownGitTest()
}

func TestGetGitChangedFolders_DummyRepo(t *testing.T) {
	setupLog()
	scenarios := []struct {
		name                      string
		beforeCommitExclusive     string
		targetCommit              string
		configFile                string
		configBranch              string
		expectedChangedFolders    []string
		expectedChangedConfigFile bool
		expectedError             string
	}{
		{
			name:                      "init - add radixconfig and gitignore files",
			targetCommit:              "7d6309f7537baa2815bb631802e6d8d613150c52",
			beforeCommitExclusive:     "",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"."},
			expectedChangedConfigFile: true,
		},
		{
			name:                      "added app1 folder and its files. app1 component added to the radixconfig",
			targetCommit:              "0b9ee1f93639fff492c05b8d5e662301f508debe",
			beforeCommitExclusive:     "7d6309f7537baa2815bb631802e6d8d613150c52",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{".", "app1"},
			expectedChangedConfigFile: true,
		},
		{
			name:                      "Changed radixconfig, but config branch is different",
			targetCommit:              "0b9ee1f93639fff492c05b8d5e662301f508debe",
			beforeCommitExclusive:     "7d6309f7537baa2815bb631802e6d8d613150c52",
			configFile:                "radixconfig.yaml",
			configBranch:              "another-branch",
			expectedChangedFolders:    []string{".", "app1"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "changed files in the folder app1",
			targetCommit:              "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			beforeCommitExclusive:     "0b9ee1f93639fff492c05b8d5e662301f508debe",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app1"},
			expectedChangedConfigFile: false,
		},
		{
			name:                  "invalid the same target and before commit",
			targetCommit:          "7d6309f7537baa2815bb631802e6d8d613150c52",
			beforeCommitExclusive: "7d6309f7537baa2815bb631802e6d8d613150c52",
			configFile:            "radixconfig.yaml",
			configBranch:          "main",
			expectedError:         "beforeCommit cannot be equal to the targetCommit",
		},
		{
			name:                  "invalid target commit",
			targetCommit:          "invalid-commit",
			beforeCommitExclusive: "",
			configFile:            "radixconfig.yaml",
			configBranch:          "main",
			expectedError:         "invalid targetCommit",
		},
		{
			name:                  "invalid empty target commit",
			targetCommit:          "",
			beforeCommitExclusive: "",
			configFile:            "radixconfig.yaml",
			configBranch:          "main",
			expectedError:         "invalid targetCommit",
		},
		{
			name:                      "Added folder app2 with files",
			targetCommit:              "157014b59d6b24205b4fbf57165f0029c49d7963",
			beforeCommitExclusive:     "f68e88664ed51f79880b7f69d5789d21086ed1dc",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Folder app2 was renamed to app3, file in folder 3 was changed",
			targetCommit:              "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:     "157014b59d6b24205b4fbf57165f0029c49d7963",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app2", "app3"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "File in folder 3 was changed",
			targetCommit:              "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:     "8d99f5318a7164ff1e03bd4fbe1cba554e9c906b",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "File in folder 3 was changed",
			targetCommit:              "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			beforeCommitExclusive:     "8d99f5318a7164ff1e03bd4fbe1cba554e9c906b",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "radixconfig.yaml was added to the folder app3",
			targetCommit:              "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			beforeCommitExclusive:     "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "radixconfig.yaml was added to the folder app3, it is current config",
			targetCommit:              "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			beforeCommitExclusive:     "31472b8fc3fe22a2b8d174e79ae2f891a975864d",
			configFile:                "app3/radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: true,
		},
		{
			name:                      "radixconfig.yaml was changed to the folder app3",
			targetCommit:              "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			beforeCommitExclusive:     "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "radixconfig.yaml was changed to the folder app3, it is current config",
			targetCommit:              "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			beforeCommitExclusive:     "d79cc7f32f58e30b01671b8bcc19f41508db95c8",
			configFile:                "app3/radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3"},
			expectedChangedConfigFile: true,
		},
		{
			name:                      "Sub-folders were added to the folder app3, with file. Sub-folder was added to the folder app1, without file.",
			targetCommit:              "13bd8316267d6a0be44f3f87fe49e807e7bc24b4",
			beforeCommitExclusive:     "0143afb54d8f9e5451b2dc34f47d7f933080e7a4",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3/data/level2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Files added and changed in the sub-folders of app3. File was added to the sub-folder in the folder app1.",
			targetCommit:              "986065b74c8e9e4012287fdd6b13021591ce00c3",
			beforeCommitExclusive:     "13bd8316267d6a0be44f3f87fe49e807e7bc24b4",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3", "app3/data", "app3/data/level2", "app1/data"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "radixconfig-app3-level2.yaml was added to the subfolder of the folder app3",
			targetCommit:              "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:     "986065b74c8e9e4012287fdd6b13021591ce00c3",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3/data/level2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "radixconfig-app3-level2.yaml was added to the subfolder of the folder app3, it is current config",
			targetCommit:              "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:     "986065b74c8e9e4012287fdd6b13021591ce00c3",
			configFile:                "app3/data/level2/radixconfig-app3-level2.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3/data/level2"},
			expectedChangedConfigFile: true,
		},
		{
			name:                      "radixconfig-app3-level2.yaml was added to the subfolder of the folder app3, it is current config, but not this branch",
			targetCommit:              "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			beforeCommitExclusive:     "986065b74c8e9e4012287fdd6b13021591ce00c3",
			configFile:                "app3/data/level2/radixconfig-app3-level2.yaml",
			configBranch:              "another-branch",
			expectedChangedFolders:    []string{"app3/data/level2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Files were changed in subfolders app1 and app3, with config radixconfig.yaml",
			targetCommit:              "2127927fa21ae471baefbadd3f05b60a4bf38b5f",
			beforeCommitExclusive:     "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			configFile:                "radixconfig.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app1/data", "app3/data/level2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Files were changed in subfolders app1 and app3, with config app3/data/level2/radixconfig-app3-level2.yaml",
			targetCommit:              "2127927fa21ae471baefbadd3f05b60a4bf38b5f",
			beforeCommitExclusive:     "e89ac3d3ba66498cf6165e119d29a86b6b8183ab",
			configFile:                "app3/data/level2/radixconfig-app3-level2.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app1/data", "app3/data/level2"},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Empty commit, made with 'git commit -m 'empty' --allow-empty",
			targetCommit:              "95b2136dd7f8bd8ec52ef46897d55ba321da3abe",
			beforeCommitExclusive:     "cdbd98a036ae5a96f5ad97d2fb225aa5e550f4c1",
			configFile:                "app3/data/level2/radixconfig-app3-level2.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Changes of in-root file, used in the component with root source",
			targetCommit:              "8b5f81bd11c1670a6894cd14abcb1c9d2cde6e7e",
			beforeCommitExclusive:     "d897c65f4322f4dc71b33ff2fd6b365f815a2026",
			configFile:                "radixconfig-in-root.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"."},
			expectedChangedConfigFile: false,
		},
		{
			name:                      "Changes of subfolder, used in the component with root source",
			targetCommit:              "192e173dc968368a5dc9f42abbcbd9dab6484194",
			beforeCommitExclusive:     "8b5f81bd11c1670a6894cd14abcb1c9d2cde6e7e",
			configFile:                "radixconfig-in-root.yaml",
			configBranch:              "main",
			expectedChangedFolders:    []string{"app3/data/level2"},
			expectedChangedConfigFile: false,
		},
	}

	gitWorkspacePath := setupGitTest("test-data-git-commits.zip", "test-data-git-commits")
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			changedFolderList, changedConfigFile, err := getGitAffectedResourcesBetweenCommits(gitWorkspacePath, scenario.configBranch, scenario.configFile, false, scenario.targetCommit, scenario.beforeCommitExclusive)
			if scenario.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, scenario.expectedError, err.Error())
			}
			assert.ElementsMatch(t, scenario.expectedChangedFolders, changedFolderList)
			assert.Equal(t, scenario.expectedChangedConfigFile, changedConfigFile)
		})
	}
	tearDownGitTest()
}

func setupLog() {
	logger.InitLogger(zerolog.DebugLevel.String())
}
