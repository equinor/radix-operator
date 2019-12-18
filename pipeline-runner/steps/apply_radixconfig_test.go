package steps

import (
	"fmt"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/stretchr/testify/assert"
)

func TestGetComponentImages_ReturnsProperMapping(t *testing.T) {
	applicationComponents := []v1.RadixComponent{
		utils.AnApplicationComponent().
			WithName("client-component-1").
			WithSourceFolder("./client/").
			WithDockerfileName("client.Dockerfile").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("client-component-2").
			WithSourceFolder("./client/").
			WithDockerfileName("client.Dockerfile").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("server-component-1").
			WithSourceFolder("./server/").
			WithDockerfileName("server.Dockerfile").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("server-component-2").
			WithSourceFolder("./server/").
			WithDockerfileName("server.Dockerfile").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("single-component").
			WithSourceFolder(".").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("public-image-component").
			WithImage("swaggerapi/swagger-ui").
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("private-hub-component").
			WithImage("radixcanary.azurecr.io/nginx:latest").
			BuildComponent()}

	anyAppName := "any-app"
	anyContainerRegistry := "any-reg"
	anyImageTag := "any-tag"

	componentImages := getComponentImages(anyAppName, anyContainerRegistry, anyImageTag, applicationComponents)

	assert.Equal(t, "build-multi-component", componentImages["client-component-1"].ContainerName)
	assert.True(t, componentImages["client-component-1"].Build)
	assert.True(t, componentImages["client-component-1"].Scan)
	assert.Equal(t, "/workspace/client/", componentImages["client-component-1"].Context)
	assert.Equal(t, "client.Dockerfile", componentImages["client-component-1"].Dockerfile)
	assert.Equal(t, "multi-component", componentImages["client-component-1"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component", anyImageTag), componentImages["client-component-1"].ImagePath)

	assert.Equal(t, "build-multi-component", componentImages["client-component-2"].ContainerName)
	assert.True(t, componentImages["client-component-2"].Build)
	assert.True(t, componentImages["client-component-2"].Scan)
	assert.Equal(t, "/workspace/client/", componentImages["client-component-2"].Context)
	assert.Equal(t, "client.Dockerfile", componentImages["client-component-2"].Dockerfile)
	assert.Equal(t, "multi-component", componentImages["client-component-2"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component", anyImageTag), componentImages["client-component-2"].ImagePath)

	assert.Equal(t, "build-multi-component-1", componentImages["server-component-1"].ContainerName)
	assert.True(t, componentImages["server-component-1"].Build)
	assert.True(t, componentImages["server-component-1"].Scan)
	assert.Equal(t, "/workspace/server/", componentImages["server-component-1"].Context)
	assert.Equal(t, "server.Dockerfile", componentImages["server-component-1"].Dockerfile)
	assert.Equal(t, "multi-component-1", componentImages["server-component-1"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-1", anyImageTag), componentImages["server-component-1"].ImagePath)

	assert.Equal(t, "build-multi-component-1", componentImages["server-component-2"].ContainerName)
	assert.True(t, componentImages["server-component-2"].Build)
	assert.True(t, componentImages["server-component-2"].Scan)
	assert.Equal(t, "/workspace/server/", componentImages["server-component-2"].Context)
	assert.Equal(t, "server.Dockerfile", componentImages["server-component-2"].Dockerfile)
	assert.Equal(t, "multi-component-1", componentImages["server-component-2"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-1", anyImageTag), componentImages["server-component-2"].ImagePath)

	assert.Equal(t, "build-single-component", componentImages["single-component"].ContainerName)
	assert.True(t, componentImages["single-component"].Build)
	assert.True(t, componentImages["single-component"].Scan)
	assert.Equal(t, "/workspace/", componentImages["single-component"].Context)
	assert.Equal(t, "Dockerfile", componentImages["single-component"].Dockerfile)
	assert.Equal(t, "single-component", componentImages["single-component"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "single-component", anyImageTag), componentImages["single-component"].ImagePath)

	assert.Equal(t, "", componentImages["public-image-component"].ContainerName)
	assert.False(t, componentImages["public-image-component"].Build)
	assert.False(t, componentImages["public-image-component"].Scan)
	assert.Equal(t, "swaggerapi/swagger-ui", componentImages["public-image-component"].ImageName)
	assert.Equal(t, "swaggerapi/swagger-ui", componentImages["public-image-component"].ImagePath)

	assert.Equal(t, "", componentImages["private-hub-component"].ContainerName)
	assert.False(t, componentImages["private-hub-component"].Build)
	assert.False(t, componentImages["private-hub-component"].Scan)
	assert.Equal(t, "radixcanary.azurecr.io/nginx:latest", componentImages["private-hub-component"].ImageName)
	assert.Equal(t, "radixcanary.azurecr.io/nginx:latest", componentImages["private-hub-component"].ImagePath)

}

func Test_dockerfile_from_build_folder(t *testing.T) {
	dockerfile := getDockerfile(".", "")

	assert.Equal(t, fmt.Sprintf("%s/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_2(t *testing.T) {
	dockerfile := getDockerfile("afolder", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_special_char(t *testing.T) {
	dockerfile := getDockerfile("./afolder/", "")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile", git.Workspace), dockerfile)
}

func Test_dockerfile_from_folder_and_file(t *testing.T) {
	dockerfile := getDockerfile("/afolder/", "Dockerfile.adockerfile")

	assert.Equal(t, fmt.Sprintf("%s/afolder/Dockerfile.adockerfile", git.Workspace), dockerfile)
}
