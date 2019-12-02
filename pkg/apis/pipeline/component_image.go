package pipeline

// ComponentImage Holds info about the image associated with a component
type ComponentImage struct {
	BuildContainerName string
	ContainerRegistry  string
	ImageName          string
	ImagePath          string
}
