package pipeline

// ContainerOutputName holds information about the configmap a container writes output to.
// Key is the name of the container and value is the name of the configmap.
// The configmap must exist in the same namespace as the container Pod.
type ContainerOutputName map[string]string
