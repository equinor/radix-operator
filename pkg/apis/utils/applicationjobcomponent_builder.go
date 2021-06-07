package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixApplicationJobComponentBuilder Handles construction of RA job component
type RadixApplicationJobComponentBuilder interface {
	WithName(string) RadixApplicationJobComponentBuilder
	WithSourceFolder(string) RadixApplicationJobComponentBuilder
	WithDockerfileName(string) RadixApplicationJobComponentBuilder
	WithImage(string) RadixApplicationJobComponentBuilder
	WithPort(string, int32) RadixApplicationJobComponentBuilder
	WithSecrets(...string) RadixApplicationJobComponentBuilder
	WithEnvironmentConfig(RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder
	WithEnvironmentConfigs(...RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder
	WithCommonEnvironmentVariable(string, string) RadixApplicationJobComponentBuilder
	WithCommonResource(map[string]string, map[string]string) RadixApplicationJobComponentBuilder
	WithSchedulerPort(*int32) RadixApplicationJobComponentBuilder
	WithPayloadPath(*string) RadixApplicationJobComponentBuilder
	WithNode(node v1.RadixNode) RadixApplicationJobComponentBuilder
	BuildJobComponent() v1.RadixJobComponent
}

type radixApplicationJobComponentBuilder struct {
	name              string
	sourceFolder      string
	dockerfileName    string
	image             string
	ports             map[string]int32
	secrets           []string
	environmentConfig []RadixJobComponentEnvironmentConfigBuilder
	variables         v1.EnvVarsMap
	resources         v1.ResourceRequirements
	schedulerPort     *int32
	payloadPath       *string
	node              v1.RadixNode
}

func (rcb *radixApplicationJobComponentBuilder) WithName(name string) RadixApplicationJobComponentBuilder {
	rcb.name = name
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithSourceFolder(sourceFolder string) RadixApplicationJobComponentBuilder {
	rcb.sourceFolder = sourceFolder
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithDockerfileName(dockerfileName string) RadixApplicationJobComponentBuilder {
	rcb.dockerfileName = dockerfileName
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithImage(image string) RadixApplicationJobComponentBuilder {
	rcb.image = image
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithSecrets(secrets ...string) RadixApplicationJobComponentBuilder {
	rcb.secrets = secrets
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithPort(name string, port int32) RadixApplicationJobComponentBuilder {
	if rcb.ports == nil {
		rcb.ports = make(map[string]int32)
	}

	rcb.ports[name] = port
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithEnvironmentConfig(environmentConfig RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder {
	rcb.environmentConfig = append(rcb.environmentConfig, environmentConfig)
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithEnvironmentConfigs(environmentConfigs ...RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder {
	rcb.environmentConfig = environmentConfigs
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithCommonEnvironmentVariable(name, value string) RadixApplicationJobComponentBuilder {
	if rcb.variables == nil {
		rcb.variables = make(v1.EnvVarsMap)
	}

	rcb.variables[name] = value
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithCommonResource(request map[string]string, limit map[string]string) RadixApplicationJobComponentBuilder {
	rcb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithSchedulerPort(port *int32) RadixApplicationJobComponentBuilder {
	rcb.schedulerPort = port
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithPayloadPath(path *string) RadixApplicationJobComponentBuilder {
	rcb.payloadPath = path
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithNode(node v1.RadixNode) RadixApplicationJobComponentBuilder {
	rcb.node = node
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) BuildJobComponent() v1.RadixJobComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range rcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	var environmentConfig = make([]v1.RadixJobComponentEnvironmentConfig, 0)
	for _, env := range rcb.environmentConfig {
		environmentConfig = append(environmentConfig, env.BuildEnvironmentConfig())
	}

	var payload *v1.RadixJobComponentPayload
	if rcb.payloadPath != nil {
		payload = &v1.RadixJobComponentPayload{Path: *rcb.payloadPath}
	}

	return v1.RadixJobComponent{
		Name:              rcb.name,
		SourceFolder:      rcb.sourceFolder,
		DockerfileName:    rcb.dockerfileName,
		Image:             rcb.image,
		Ports:             componentPorts,
		Secrets:           rcb.secrets,
		EnvironmentConfig: environmentConfig,
		Variables:         rcb.variables,
		Resources:         rcb.resources,
		SchedulerPort:     rcb.schedulerPort,
		Payload:           payload,
		Node:              rcb.node,
	}
}

// NewApplicationJobComponentBuilder Constructor for job component builder
func NewApplicationJobComponentBuilder() RadixApplicationJobComponentBuilder {
	return &radixApplicationJobComponentBuilder{}
}

// AnApplicationJobComponent Constructor for job component builder containing test data
func AnApplicationJobComponent() RadixApplicationJobComponentBuilder {
	return &radixApplicationJobComponentBuilder{
		name: "job",
	}
}
