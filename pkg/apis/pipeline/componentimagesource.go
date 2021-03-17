package pipeline

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// ComponentImageSource holds information about the container image source for a Radix component or job
type ComponentImageSource struct {
	Name           string
	SourceFolder   string
	DockerfileName string
	Image          string
}

// SourceFunc defines a function that can be used as input argument to WithSourceFunc
type SourceFunc func(*componentImageSourceBuilder)

// RadixComponentSource returns a SourceFunc that sets properties from the RadixComponent
func RadixComponentSource(c v1.RadixComponent) SourceFunc {
	return func(b *componentImageSourceBuilder) {
		b.WithName(c.Name).
			WithSourceFolder(c.SourceFolder).
			WithDockerfileName(c.DockerfileName).
			WithImage(c.Image)
	}
}

// RadixJobComponentSource returns a SourceFunc that sets properties from the RadixJobComponent
func RadixJobComponentSource(c v1.RadixJobComponent) SourceFunc {
	return func(b *componentImageSourceBuilder) {
		b.WithName(c.Name).
			WithSourceFolder(c.SourceFolder).
			WithDockerfileName(c.DockerfileName).
			WithImage(c.Image)
	}
}

// ComponentImageSourceBuilder handles construction of ComponentImageSource
type ComponentImageSourceBuilder interface {
	WithName(name string) ComponentImageSourceBuilder
	WithSourceFolder(sourceFolder string) ComponentImageSourceBuilder
	WithDockerfileName(dockerfileName string) ComponentImageSourceBuilder
	WithImage(image string) ComponentImageSourceBuilder
	WithSourceFunc(f SourceFunc) ComponentImageSourceBuilder
	Build() ComponentImageSource
}

type componentImageSourceBuilder struct {
	name           string
	sourceFolder   string
	dockerfileName string
	image          string
}

// NewComponentImageSourceBuilder Constructor for the ComponentImageSourceBuilder
func NewComponentImageSourceBuilder() ComponentImageSourceBuilder {
	return &componentImageSourceBuilder{}
}

// WithName sets the name of the component image
func (b *componentImageSourceBuilder) WithName(name string) ComponentImageSourceBuilder {
	b.name = name
	return b
}

// WithSourceFolder sets the name of the directory where the docker file exists
func (b *componentImageSourceBuilder) WithSourceFolder(sourceFolder string) ComponentImageSourceBuilder {
	b.sourceFolder = sourceFolder
	return b
}

// WithDockerfileName sets the name of the docker file
func (b *componentImageSourceBuilder) WithDockerfileName(dockerfileName string) ComponentImageSourceBuilder {
	b.dockerfileName = dockerfileName
	return b
}

// WithImage set the image to use as source for the component
func (b *componentImageSourceBuilder) WithImage(image string) ComponentImageSourceBuilder {
	b.image = image
	return b
}

// WithSourceFunc calls the SourceFunc to set properties from a source object
func (b *componentImageSourceBuilder) WithSourceFunc(f SourceFunc) ComponentImageSourceBuilder {
	f(b)
	return b
}

// Build builds the ComponentImageSource
func (b *componentImageSourceBuilder) Build() ComponentImageSource {
	return ComponentImageSource{
		Name:           b.name,
		SourceFolder:   b.sourceFolder,
		DockerfileName: b.dockerfileName,
		Image:          b.image,
	}
}
