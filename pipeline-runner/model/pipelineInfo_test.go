package model

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	"github.com/stretchr/testify/assert"
)

var (
	copyConfigToMap = &DefaultStepImplementation{StepType: pipeline.CopyConfigToMapStep, SuccessMessage: "config copied to map"}
	applyConfigStep = &DefaultStepImplementation{StepType: pipeline.ApplyConfigStep, SuccessMessage: "config applied"}
	buildStep       = &DefaultStepImplementation{StepType: pipeline.BuildStep, SuccessMessage: "built"}
	scanImageStep   = &DefaultStepImplementation{StepType: pipeline.ScanImageStep, SuccessMessage: "image scanned"}
	deployStep      = &DefaultStepImplementation{StepType: pipeline.DeployStep, SuccessMessage: "deployed"}
)

func Test_DefaultPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName("")
	p, _ := InitPipeline(pipelineType, PipelineArguments{}, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildDeployPipeType(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	p, _ := InitPipeline(pipelineType, PipelineArguments{}, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)

	assert.Equal(t, v1.BuildDeploy, p.Definition.Type)
	assert.Equal(t, 5, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[4].SucceededMsg())
}

func Test_BuildAndDefaultPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := GetPipelineArgsFromArguments(make(map[string]string))
	p, _ := InitPipeline(pipelineType, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.True(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_BuildOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := PipelineArguments{
		PushImage: false,
	}

	p, _ := InitPipeline(pipelineType, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.False(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_BuildAndPushOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Build))

	pipelineArgs := PipelineArguments{
		PushImage: true,
	}

	p, _ := InitPipeline(pipelineType, pipelineArgs, copyConfigToMap, applyConfigStep, buildStep, scanImageStep, deployStep)
	assert.Equal(t, v1.Build, p.Definition.Type)
	assert.True(t, p.PipelineArguments.PushImage)
	assert.Equal(t, 4, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "built", p.Steps[2].SucceededMsg())
	assert.Equal(t, "image scanned", p.Steps[3].SucceededMsg())
}

func Test_DeployOnlyPipeline(t *testing.T) {
	pipelineType, _ := pipeline.GetPipelineFromName(string(v1.Deploy))

	toEnvironment := "dev"
	pipelineArgs := PipelineArguments{
		ToEnvironment: toEnvironment,
	}

	p, _ := InitPipeline(pipelineType, pipelineArgs, copyConfigToMap, applyConfigStep, deployStep)
	assert.Equal(t, v1.Deploy, p.Definition.Type)
	assert.Equal(t, toEnvironment, p.PipelineArguments.ToEnvironment)
	assert.Equal(t, 3, len(p.Steps))
	assert.Equal(t, "config copied to map", p.Steps[0].SucceededMsg())
	assert.Equal(t, "config applied", p.Steps[1].SucceededMsg())
	assert.Equal(t, "deployed", p.Steps[2].SucceededMsg())
}

func Test_NonExistingPipelineType(t *testing.T) {
	_, err := pipeline.GetPipelineFromName("non existing pipeline")
	assert.NotNil(t, err)
}

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
			BuildComponent(),
		utils.AnApplicationComponent().
			WithName("compute-shared-1").
			WithSourceFolder("./compute/").
			WithDockerfileName("compute.Dockerfile").
			BuildComponent(),
	}

	jobComponents := []v1.RadixJobComponent{
		utils.AnApplicationJobComponent().
			WithName("compute-shared-2").
			WithDockerfileName("compute.Dockerfile").
			WithSourceFolder("./compute/").
			BuildJobComponent(),
		utils.AnApplicationJobComponent().
			WithName("single-job").
			WithDockerfileName("job.Dockerfile").
			WithSourceFolder("./job/").
			BuildJobComponent(),
		utils.AnApplicationJobComponent().
			WithName("calc-1").
			WithDockerfileName("calc.Dockerfile").
			WithSourceFolder("./calc/").
			BuildJobComponent(),
		utils.AnApplicationJobComponent().
			WithName("calc-2").
			WithDockerfileName("calc.Dockerfile").
			WithSourceFolder("./calc/").
			BuildJobComponent(),
		utils.AnApplicationJobComponent().
			WithName("public-job-component").
			WithImage("job/job:latest").
			BuildJobComponent(),
	}

	anyAppName := "any-app"
	anyContainerRegistry := "any-reg"
	anyImageTag := "any-tag"

	componentImages := getComponentImages(anyAppName, anyContainerRegistry, anyImageTag, applicationComponents, jobComponents)

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

	assert.Equal(t, "build-multi-component-2", componentImages["compute-shared-1"].ContainerName)
	assert.True(t, componentImages["compute-shared-1"].Build)
	assert.True(t, componentImages["compute-shared-1"].Scan)
	assert.Equal(t, "/workspace/compute/", componentImages["compute-shared-1"].Context)
	assert.Equal(t, "compute.Dockerfile", componentImages["compute-shared-1"].Dockerfile)
	assert.Equal(t, "multi-component-2", componentImages["compute-shared-1"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-2", anyImageTag), componentImages["compute-shared-1"].ImagePath)

	assert.Equal(t, "build-multi-component-2", componentImages["compute-shared-2"].ContainerName)
	assert.True(t, componentImages["compute-shared-2"].Build)
	assert.True(t, componentImages["compute-shared-2"].Scan)
	assert.Equal(t, "/workspace/compute/", componentImages["compute-shared-2"].Context)
	assert.Equal(t, "compute.Dockerfile", componentImages["compute-shared-2"].Dockerfile)
	assert.Equal(t, "multi-component-2", componentImages["compute-shared-2"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-2", anyImageTag), componentImages["compute-shared-2"].ImagePath)

	assert.Equal(t, "build-single-job", componentImages["single-job"].ContainerName)
	assert.True(t, componentImages["single-job"].Build)
	assert.True(t, componentImages["single-job"].Scan)
	assert.Equal(t, "/workspace/job/", componentImages["single-job"].Context)
	assert.Equal(t, "job.Dockerfile", componentImages["single-job"].Dockerfile)
	assert.Equal(t, "single-job", componentImages["single-job"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "single-job", anyImageTag), componentImages["single-job"].ImagePath)

	assert.Equal(t, "build-multi-component-3", componentImages["calc-1"].ContainerName)
	assert.True(t, componentImages["calc-1"].Build)
	assert.True(t, componentImages["calc-1"].Scan)
	assert.Equal(t, "/workspace/calc/", componentImages["calc-1"].Context)
	assert.Equal(t, "calc.Dockerfile", componentImages["calc-1"].Dockerfile)
	assert.Equal(t, "multi-component-3", componentImages["calc-1"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-3", anyImageTag), componentImages["calc-1"].ImagePath)

	assert.Equal(t, "build-multi-component-3", componentImages["calc-2"].ContainerName)
	assert.True(t, componentImages["calc-2"].Build)
	assert.True(t, componentImages["calc-2"].Scan)
	assert.Equal(t, "/workspace/calc/", componentImages["calc-2"].Context)
	assert.Equal(t, "calc.Dockerfile", componentImages["calc-2"].Dockerfile)
	assert.Equal(t, "multi-component-3", componentImages["calc-2"].ImageName)
	assert.Equal(t, utils.GetImagePath(anyContainerRegistry, anyAppName, "multi-component-3", anyImageTag), componentImages["calc-2"].ImagePath)

	assert.Equal(t, "", componentImages["public-job-component"].ContainerName)
	assert.False(t, componentImages["public-job-component"].Build)
	assert.False(t, componentImages["public-job-component"].Scan)
	assert.Equal(t, "job/job:latest", componentImages["public-job-component"].ImageName)
	assert.Equal(t, "job/job:latest", componentImages["public-job-component"].ImagePath)
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

func Test_IsDeployOnlyPipeline(t *testing.T) {
	toEnvironment := "prod"
	pipelineArguments := PipelineArguments{
		ToEnvironment: toEnvironment,
	}

	pipelineInfo := PipelineInfo{
		PipelineArguments: pipelineArguments,
	}

	assert.True(t, pipelineInfo.IsDeployOnlyPipeline())

	fromEnvironment := "dev"
	pipelineArguments = PipelineArguments{
		ToEnvironment:   toEnvironment,
		FromEnvironment: fromEnvironment,
	}

	pipelineInfo = PipelineInfo{
		PipelineArguments: pipelineArguments,
	}

	assert.False(t, pipelineInfo.IsDeployOnlyPipeline())
}
