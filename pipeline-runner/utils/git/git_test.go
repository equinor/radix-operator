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

func TestGetGitChangedFolders_DummyRepo(t *testing.T) {
	setupLog()
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
			expectedError:         fmt.Errorf("object not found"),
		},
		"invalid empty target commit": {
			targetCommit:          "",
			beforeCommitExclusive: "",
			expectedError:         fmt.Errorf("invalid empty targetCommit"),
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
		for scenarioName, scenario := range scenarios {
			t.Run(fmt.Sprintf("%s (zip: %s, folder: %s)", scenarioName, zipFile, folder), func(t *testing.T) {
				repo, err := Open(gitWorkspacePath)
				require.NoError(t, err)
				diffs, err := repo.DiffCommits(scenario.beforeCommitExclusive, scenario.targetCommit)
				if scenario.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, scenario.expectedError)
					assert.ElementsMatch(t, scenario.expectedChangedFolders, diffs.Dirs())
				}

				// var changedFolderList, changedConfigFile, err = getGitAffectedResourcesBetweenCommits(gitWorkspacePath, scenario.configBranch, scenario.configFile, scenario.targetCommit, scenario.beforeCommitExclusive)
				// if scenario.expectedError == "" {
				// 	require.NoError(t, err)
				// } else {
				// 	require.Error(t, err)
				// 	require.Equal(t, scenario.expectedError, err.Error())
				// }
				// assert.ElementsMatch(t, scenario.expectedChangedFolders, changedFolderList, "Unexpected changed folder list")
				// assert.Equal(t, scenario.expectedChangedConfigFile, changedConfigFile, "Unexpected changed config file")
			})
		}
		tearDownGitTest()
	}
}

func setupLog() {
	logger.InitLogger(zerolog.DebugLevel.String())
}
