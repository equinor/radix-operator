package onpush

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_dockerfile_from_build_folder(t *testing.T) {
	dockerfile := getDockerfile(".", "")

	assert.Equal(t, fmt.Sprintf("%s/Dockerfile", workspace), dockerfile)
}

func Test_dockerfile_from_folder(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", workspace), dockerfile)
}

func Test_dockerfile_from_folder_2(t *testing.T) {
	dockerfile := getDockerfile("afolder", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", workspace), dockerfile)
}

func Test_dockerfile_from_folder_special_char(t *testing.T) {
	dockerfile := getDockerfile("./afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", workspace), dockerfile)
}

func Test_dockerfile_from_folder_and_file(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "Dockerfile.adockerfile")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile.adockerfile", workspace), dockerfile)
}

func Test_getGitCloneCommand(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")

	assert.Equal(t, "git clone git@github.com:equinor/radix-github-webhook.git -b master .", gitCloneCommand)
}

func Test_getInitContainerArgString_NoCommitID(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")
	argString := getInitContainerArgString("/workspace", gitCloneCommand, "")

	assert.Equal(t, "apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && git clone git@github.com:equinor/radix-github-webhook.git -b master .", argString)
}

func Test_getInitContainerArgString_WithCommitID(t *testing.T) {
	gitCloneCommand := getGitCloneCommand("git@github.com:equinor/radix-github-webhook.git", "master")
	argString := getInitContainerArgString("/workspace", gitCloneCommand, "762235917d04dd541715df25a428b5f62394834c")

	assert.Equal(t, "apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && git clone git@github.com:equinor/radix-github-webhook.git -b master . && git checkout 762235917d04dd541715df25a428b5f62394834c", argString)
}
