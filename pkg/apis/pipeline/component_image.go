package pipeline

// ComponentImage Holds info about the image associated with a component
type ComponentImage struct {
	ContainerName string
	Context       string
	Dockerfile    string
	ImageName     string
	ImagePath     string
	ImageTag      string
	Build         bool
}
