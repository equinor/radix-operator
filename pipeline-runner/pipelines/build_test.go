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
