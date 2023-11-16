package ingress

// IngressConfiguration Holds all ingress annotation configurations
type IngressConfiguration struct {
	AnnotationConfigurations []AnnotationConfiguration `yaml:"configuration"`
}

// AnnotationConfiguration Holds annotations for a single configuration
type AnnotationConfiguration struct {
	Name        string
	Annotations map[string]string
}
