package pipeline

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// DeployComponentImage Holds info about the image associated with a component
type DeployComponentImage struct {
	ImagePath    string
	ImageTagName string
	Runtime      *radixv1.Runtime
	Build        bool
}

// DeployComponentImages maps component names with image information
type DeployComponentImages map[string]DeployComponentImage

// DeployEnvironmentComponentImages maps environment names with components to be deployed
type DeployEnvironmentComponentImages map[string]DeployComponentImages

// BuildComponentImage holds info about a build container
type BuildComponentImage struct {
	ComponentName string
	EnvName       string
	ContainerName string
	Context       string
	Dockerfile    string
	ImageName     string
	ImagePath     string
	Runtime       *radixv1.Runtime
}

// EnvironmentBuildComponentImages maps component names with build information for environment
type EnvironmentBuildComponentImages map[string][]BuildComponentImage

// BuildComponentImages maps component names with build information
type BuildComponentImages map[string]BuildComponentImage
