package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixApplicationComponentBuilder Handles construction of RA component
type RadixApplicationComponentBuilder interface {
	WithName(string) RadixApplicationComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) RadixApplicationComponentBuilder
	WithSourceFolder(string) RadixApplicationComponentBuilder
	WithDockerfileName(string) RadixApplicationComponentBuilder
	WithImage(string) RadixApplicationComponentBuilder
	WithPublic(bool) RadixApplicationComponentBuilder // Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublicPort(string) RadixApplicationComponentBuilder
	WithPort(string, int32) RadixApplicationComponentBuilder
	WithPorts([]v1.ComponentPort) RadixApplicationComponentBuilder
	WithSecrets(...string) RadixApplicationComponentBuilder
	WithSecretRefs(v1.RadixSecretRefs) RadixApplicationComponentBuilder
	WithMonitoringConfig(v1.MonitoringConfig) RadixApplicationComponentBuilder
	WithIngressConfiguration(...string) RadixApplicationComponentBuilder
	WithEnvironmentConfig(RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithEnvironmentConfigs(...RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithCommonEnvironmentVariable(string, string) RadixApplicationComponentBuilder
	WithCommonResource(map[string]string, map[string]string) RadixApplicationComponentBuilder
	WithNode(v1.RadixNode) RadixApplicationComponentBuilder
	WithAuthentication(*v1.Authentication) RadixApplicationComponentBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) RadixApplicationComponentBuilder
	WithEnabled(bool) RadixApplicationComponentBuilder
	WithIdentity(*v1.Identity) RadixApplicationComponentBuilder
	BuildComponent() v1.RadixComponent
}

type radixApplicationComponentBuilder struct {
	name                    string
	sourceFolder            string
	dockerfileName          string
	image                   string
	alwaysPullImageOnDeploy *bool
	public                  bool // Deprecated: For backwards compatibility public is still supported, new code should use publicPort instead
	publicPort              string
	monitoringConfig        v1.MonitoringConfig
	ports                   []v1.ComponentPort
	secrets                 []string
	secretRefs              v1.RadixSecretRefs
	ingressConfiguration    []string
	environmentConfig       []RadixEnvironmentConfigBuilder
	variables               v1.EnvVarsMap
	resources               v1.ResourceRequirements
	node                    v1.RadixNode
	authentication          *v1.Authentication
	volumeMounts            []v1.RadixVolumeMount
	enabled                 *bool
	identity                *v1.Identity
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

func (rcb *radixApplicationComponentBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) RadixApplicationComponentBuilder {
	rcb.secretRefs = secretRefs
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithMonitoringConfig(monitoringConfig v1.MonitoringConfig) RadixApplicationComponentBuilder {
	rcb.monitoringConfig = monitoringConfig
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) RadixApplicationComponentBuilder {
	rcb.ingressConfiguration = ingressConfiguration
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPort(name string, port int32) RadixApplicationComponentBuilder {
	if rcb.ports == nil {
		rcb.ports = make([]v1.ComponentPort, 0)
	}

	rcb.ports = append(rcb.ports, v1.ComponentPort{Name: name, Port: port})
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPorts(ports []v1.ComponentPort) RadixApplicationComponentBuilder {
	for i := range ports {
		rcb.WithPort(ports[i].Name, ports[i].Port)
	}
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
	if rcb.variables == nil {
		rcb.variables = make(v1.EnvVarsMap)
	}

	rcb.variables[name] = value
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithNode(node v1.RadixNode) RadixApplicationComponentBuilder {
	rcb.node = node
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithAuthentication(authentication *v1.Authentication) RadixApplicationComponentBuilder {
	rcb.authentication = authentication
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithVolumeMounts(volumes []v1.RadixVolumeMount) RadixApplicationComponentBuilder {
	rcb.volumeMounts = volumes
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnabled(enabled bool) RadixApplicationComponentBuilder {
	rcb.enabled = &enabled
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithIdentity(identity *v1.Identity) RadixApplicationComponentBuilder {
	rcb.identity = identity
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
	var environmentConfig = make([]v1.RadixEnvironmentConfig, 0)
	for _, env := range rcb.environmentConfig {
		environmentConfig = append(environmentConfig, env.BuildEnvironmentConfig())
	}

	return v1.RadixComponent{
		Name:                    rcb.name,
		SourceFolder:            rcb.sourceFolder,
		DockerfileName:          rcb.dockerfileName,
		Image:                   rcb.image,
		Ports:                   rcb.ports,
		Secrets:                 rcb.secrets,
		SecretRefs:              rcb.secretRefs,
		IngressConfiguration:    rcb.ingressConfiguration,
		Public:                  rcb.public,
		PublicPort:              rcb.publicPort,
		MonitoringConfig:        rcb.monitoringConfig,
		EnvironmentConfig:       environmentConfig,
		Variables:               rcb.variables,
		Resources:               rcb.resources,
		AlwaysPullImageOnDeploy: rcb.alwaysPullImageOnDeploy,
		Node:                    rcb.node,
		Authentication:          rcb.authentication,
		Enabled:                 rcb.enabled,
		Identity:                rcb.identity,
	}
}

// NewApplicationComponentBuilder Constructor for component builder
func NewApplicationComponentBuilder() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{}
}

// AnApplicationComponent Constructor for component builder containing test data
func AnApplicationComponent() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{
		name: "app",
	}
}
