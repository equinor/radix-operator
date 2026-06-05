package git_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func envVarMap(envVars []corev1.EnvVar) map[string]string {
	m := make(map[string]string)
	for _, e := range envVars {
		m[e.Name] = e.Value
	}
	return m
}

func Test_CloneInitContainers_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	containers := git.CloneInitContainersWithSourceCode("anysshurl", "anybranch", "anycommit", "/some-workspace", "anygit:any")
	actual := slice.Map(containers, func(c corev1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: git.CloneContainerName, image: "anygit:any"},
	}
	assert.Equal(t, expected, actual)
}

func Test_CloneInitContainersWithContainerName_CustomImages(t *testing.T) {
	type containerInfo struct {
		name  string
		image string
	}
	cloneName := "anyclonename"
	containers := git.CloneInitContainersWithContainerName("anysshurl", "anybranch", "anycommit", "/some-workspace", true, false, cloneName, "anygit:any")
	actual := slice.Map(containers, func(c corev1.Container) containerInfo { return containerInfo{name: c.Name, image: c.Image} })
	expected := []containerInfo{
		{name: cloneName, image: "anygit:any"},
	}
	assert.Equal(t, expected, actual)
}

// Shell injection tests: user-controlled values must never be interpolated raw into the shell script.
// Each value must be stored in an env var and referenced via quoted "$VAR" in the command.

func Test_CloneInitContainers_ShellInjection_BranchName(t *testing.T) {
	maliciousBranch := `main$(echo${IFS}RADIX_CLONE_INJECTION_PROOF>&2)`
	containers := git.CloneInitContainersWithSourceCode("git@github.com:org/repo.git", maliciousBranch, "", "/workspace", "gitimage:latest")
	require.Len(t, containers, 1)
	c := containers[0]
	require.Len(t, c.Command, 3)
	// The raw branch value must not appear in the shell script
	assert.NotContains(t, c.Command[2], maliciousBranch)
	// The value must be passed via env var instead
	envs := envVarMap(c.Env)
	assert.Equal(t, maliciousBranch, envs["RADIX_CLONE_BRANCH"])
}

func Test_CloneInitContainers_ShellInjection_CommitID(t *testing.T) {
	maliciousCommit := "abc123; echo INJECTED"
	containers := git.CloneInitContainersWithSourceCode("git@github.com:org/repo.git", "main", maliciousCommit, "/workspace", "gitimage:latest")
	require.Len(t, containers, 1)
	c := containers[0]
	require.Len(t, c.Command, 3)
	assert.NotContains(t, c.Command[2], maliciousCommit)
	envs := envVarMap(c.Env)
	assert.Equal(t, maliciousCommit, envs["RADIX_CLONE_COMMIT"])
}

func Test_CloneInitContainers_ShellInjection_RepoURL(t *testing.T) {
	maliciousURL := "git@github.com:org/repo.git && echo INJECTED"
	containers := git.CloneInitContainersWithSourceCode(maliciousURL, "main", "", "/workspace", "gitimage:latest")
	require.Len(t, containers, 1)
	c := containers[0]
	require.Len(t, c.Command, 3)
	assert.NotContains(t, c.Command[2], maliciousURL)
	envs := envVarMap(c.Env)
	assert.Equal(t, maliciousURL, envs["RADIX_CLONE_REPO"])
}

func Test_CloneInitContainers_ShellInjection_Directory(t *testing.T) {
	maliciousDir := "/workspace; echo INJECTED"
	containers := git.CloneInitContainersWithSourceCode("git@github.com:org/repo.git", "main", "", maliciousDir, "gitimage:latest")
	require.Len(t, containers, 1)
	c := containers[0]
	require.Len(t, c.Command, 3)
	assert.NotContains(t, c.Command[2], maliciousDir)
	envs := envVarMap(c.Env)
	assert.Equal(t, maliciousDir, envs["RADIX_CLONE_DIR"])
}

func Test_CloneInitContainers_EnvVarsPresent(t *testing.T) {
	containers := git.CloneInitContainersWithSourceCode("git@github.com:org/repo.git", "main", "abc123def456", "/workspace", "gitimage:latest")
	require.Len(t, containers, 1)
	envs := envVarMap(containers[0].Env)
	assert.Equal(t, "git@github.com:org/repo.git", envs["RADIX_CLONE_REPO"])
	assert.Equal(t, "main", envs["RADIX_CLONE_BRANCH"])
	assert.Equal(t, "/workspace", envs["RADIX_CLONE_DIR"])
	assert.Equal(t, "abc123def456", envs["RADIX_CLONE_COMMIT"])
}

func Test_CloneInitContainers_NoCommitID_NoCommitEnvVar(t *testing.T) {
	containers := git.CloneInitContainersWithSourceCode("git@github.com:org/repo.git", "main", "", "/workspace", "gitimage:latest")
	require.Len(t, containers, 1)
	envs := envVarMap(containers[0].Env)
	value, hasCommit := envs["RADIX_CLONE_COMMIT"]
	assert.True(t, hasCommit)
	assert.Equal(t, "", value)
}
