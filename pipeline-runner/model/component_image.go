package model

import (
	"fmt"
	"path/filepath"
	"strings"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
)

type buildComponentImage struct {
	containerRegistry    string
	appContainerRegistry string
	appName              string
	componentName        string
	envName              string
	containerName        string
	context              string
	dockerfile           string
	imageTag             string
}

// NewBuildComponentImage Create BuildComponentImage instance
func NewBuildComponentImage(pipelineInfo *PipelineInfo, envName, componentName string, componentSource radixv1.ComponentSource) pipeline.BuildComponentImage {
	return &buildComponentImage{
		containerRegistry:    pipelineInfo.PipelineArguments.ContainerRegistry,
		appContainerRegistry: pipelineInfo.PipelineArguments.AppContainerRegistry,
		appName:              pipelineInfo.RadixApplication.GetName(),
		componentName:        strings.ToLower(componentName),
		envName:              envName,
		context:              getContext(componentSource.Folder),
		dockerfile:           getDockerfileName(componentSource.DockefileName),
		imageTag:             pipelineInfo.PipelineArguments.ImageTag,
	}
}

func (bci *buildComponentImage) GetComponentName() string {
	return bci.componentName
}

func (bci *buildComponentImage) GetEnvName() string {
	return bci.envName
}

func (bci *buildComponentImage) GetContainerName() string {
	envNameForName := getLengthLimitedName(bci.envName)
	return fmt.Sprintf("build-%s-%s", bci.componentName, envNameForName)
}

func (bci *buildComponentImage) GetContext() string {
	return bci.context
}

func (bci *buildComponentImage) GetDockerfile() string {
	return bci.dockerfile
}

func (bci *buildComponentImage) GetImageName() string {
	if len(bci.envName) == 0 {
		return bci.componentName
	}
	envNameForName := getLengthLimitedName(bci.envName)
	return fmt.Sprintf("%s-%s", envNameForName, bci.componentName)
}

func (bci *buildComponentImage) GetImagePath() string {
	return operatorutils.GetImagePath(bci.containerRegistry, bci.appName, bci.GetImageName(), bci.imageTag)
}

func getLengthLimitedName(name string) string {
	validatedName := strings.ToLower(name)
	if len(validatedName) > 10 {
		return fmt.Sprintf("%s-%s", validatedName[:5], strings.ToLower(commonutils.RandString(4)))
	}
	return validatedName
}

func getDockerfileName(name string) string {
	if name == "" {
		return "Dockerfile"
	}
	return name
}

func getContext(sourceFolder string) string {
	sourceRoot := filepath.Join("/", sourceFolder)
	return fmt.Sprintf("%s/", filepath.Join(git.Workspace, sourceRoot))
}
