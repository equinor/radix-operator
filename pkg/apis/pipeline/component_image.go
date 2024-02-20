package pipeline

// DeployComponentImage Holds info about the image associated with a component
type DeployComponentImage struct {
	ImagePath    string
	ImageTagName string
	Build        bool
}

// DeployComponentImages maps component names with image information
type DeployComponentImages map[string]DeployComponentImage

// DeployEnvironmentComponentImages maps environment names with components to be deployed
type DeployEnvironmentComponentImages map[string]DeployComponentImages

// BuildComponentImage holds info about a build container
type BuildComponentImage interface {
	GetComponentName() string
	GetEnvName() string
	GetContainerName() string
	GetContext() string
	GetDockerfile() string
	GetImageName() string
	GetImagePath() string
}

// EnvironmentBuildComponentImages maps component names with build information for environment
type EnvironmentBuildComponentImages map[string][]BuildComponentImage

// BuildComponentImages maps component names with build information
type BuildComponentImages map[string]BuildComponentImage
