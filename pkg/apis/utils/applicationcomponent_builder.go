package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixApplicationComponentBuilder Handles construction of RA component
type RadixApplicationComponentBuilder interface {
	WithName(string) RadixApplicationComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) RadixApplicationComponentBuilder
	WithSourceFolder(string) RadixApplicationComponentBuilder
	WithDockerfileName(string) RadixApplicationComponentBuilder
	WithImage(string) RadixApplicationComponentBuilder
	// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublic(bool) RadixApplicationComponentBuilder
	WithPublicPort(string) RadixApplicationComponentBuilder
	WithPort(string, int32) RadixApplicationComponentBuilder
	WithSecrets(...string) RadixApplicationComponentBuilder
	WithIngressConfiguration(...string) RadixApplicationComponentBuilder
	WithEnvironmentConfig(RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithEnvironmentConfigs(...RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithCommonEnvironmentVariable(string, string) RadixApplicationComponentBuilder
	WithCommonResource(map[string]string, map[string]string) RadixApplicationComponentBuilder
	BuildComponent() v1.RadixComponent
}

type radixApplicationComponentBuilder struct {
	name                    string
	sourceFolder            string
	dockerfileName          string
	image                   string
	alwaysPullImageOnDeploy *bool
	// Deprecated: For backwards comptibility public is still supported, new code should use publicPort instead
	public               bool
	publicPort           string
	ports                map[string]int32
	secrets              []string
	ingressConfiguration []string
	environmentConfig    []RadixEnvironmentConfigBuilder
	variables            v1.EnvVarsMap
	resources            v1.ResourceRequirements
}

func (rcb *radixApplicationComponentBuilder) WithName(name string) RadixApplicationComponentBuilder {
	rcb.name = name
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithAlwaysPullImageOnDeploy(val bool) RadixApplicationComponentBuilder {
	rcb.alwaysPullImageOnDeploy = &val
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithSourceFolder(sourceFolder string) RadixApplicationComponentBuilder {
	rcb.sourceFolder = sourceFolder
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithDockerfileName(dockerfileName string) RadixApplicationComponentBuilder {
	rcb.dockerfileName = dockerfileName
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithImage(image string) RadixApplicationComponentBuilder {
	rcb.image = image
	return rcb
}

// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
func (rcb *radixApplicationComponentBuilder) WithPublic(public bool) RadixApplicationComponentBuilder {
	rcb.public = public
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPublicPort(publicPort string) RadixApplicationComponentBuilder {
	rcb.publicPort = publicPort
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithSecrets(secrets ...string) RadixApplicationComponentBuilder {
	rcb.secrets = secrets
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) RadixApplicationComponentBuilder {
	rcb.ingressConfiguration = ingressConfiguration
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPort(name string, port int32) RadixApplicationComponentBuilder {
	rcb.ports[name] = port
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnvironmentConfig(environmentConfig RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder {
	rcb.environmentConfig = append(rcb.environmentConfig, environmentConfig)
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnvironmentConfigs(environmentConfigs ...RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder {
	rcb.environmentConfig = environmentConfigs
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithCommonEnvironmentVariable(name, value string) RadixApplicationComponentBuilder {
	rcb.variables[name] = value
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithCommonResource(request map[string]string, limit map[string]string) RadixApplicationComponentBuilder {
	rcb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return rcb
}

func (rcb *radixApplicationComponentBuilder) BuildComponent() v1.RadixComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range rcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	var environmentConfig = make([]v1.RadixEnvironmentConfig, 0)
	for _, env := range rcb.environmentConfig {
		environmentConfig = append(environmentConfig, env.BuildEnvironmentConfig())
	}

	return v1.RadixComponent{
		Name:                    rcb.name,
		SourceFolder:            rcb.sourceFolder,
		DockerfileName:          rcb.dockerfileName,
		Image:                   rcb.image,
		Ports:                   componentPorts,
		Secrets:                 rcb.secrets,
		IngressConfiguration:    rcb.ingressConfiguration,
		Public:                  rcb.public,
		PublicPort:              rcb.publicPort,
		EnvironmentConfig:       environmentConfig,
		Variables:               rcb.variables,
		Resources:               rcb.resources,
		AlwaysPullImageOnDeploy: rcb.alwaysPullImageOnDeploy,
	}
}

// NewApplicationComponentBuilder Constructor for component builder
func NewApplicationComponentBuilder() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{
		ports:     make(map[string]int32),
		variables: make(map[string]string),
	}
}

// AnApplicationComponent Constructor for component builder builder containing test data
func AnApplicationComponent() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{
		name:      "app",
		ports:     make(map[string]int32),
		variables: make(map[string]string),
	}
}
