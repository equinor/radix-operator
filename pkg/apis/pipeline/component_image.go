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
type BuildComponentImage struct {
	ComponentName string
	ContainerName string
	Context       string
	Dockerfile    string
	ImageName     string
	ImagePath     string
}

// EnvironmentBuildComponentImages maps component names with build information for environment
type EnvironmentBuildComponentImages map[string][]BuildComponentImage
