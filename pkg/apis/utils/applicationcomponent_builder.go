package utils

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixApplicationComponentBuilder Handles construction of RA component
type RadixApplicationComponentBuilder interface {
	WithName(string) RadixApplicationComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) RadixApplicationComponentBuilder
	WithSourceFolder(string) RadixApplicationComponentBuilder
	WithDockerfileName(string) RadixApplicationComponentBuilder
	WithImage(string) RadixApplicationComponentBuilder
	WithImageTagName(imageTagName string) RadixApplicationComponentBuilder
	WithPublic(bool) RadixApplicationComponentBuilder // Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublicPort(string) RadixApplicationComponentBuilder
	WithPort(string, int32) RadixApplicationComponentBuilder
	WithPorts([]radixv1.ComponentPort) RadixApplicationComponentBuilder
	WithSecrets(...string) RadixApplicationComponentBuilder
	WithSecretRefs(radixv1.RadixSecretRefs) RadixApplicationComponentBuilder
	WithMonitoringConfig(radixv1.MonitoringConfig) RadixApplicationComponentBuilder
	WithMonitoring(monitoring *bool) RadixApplicationComponentBuilder
	WithIngressConfiguration(...string) RadixApplicationComponentBuilder
	WithEnvironmentConfig(RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithEnvironmentConfigs(...RadixEnvironmentConfigBuilder) RadixApplicationComponentBuilder
	WithCommonEnvironmentVariable(string, string) RadixApplicationComponentBuilder
	WithCommonResource(map[string]string, map[string]string) RadixApplicationComponentBuilder
	WithNode(radixv1.RadixNode) RadixApplicationComponentBuilder
	WithAuthentication(*radixv1.Authentication) RadixApplicationComponentBuilder
	WithVolumeMounts([]radixv1.RadixVolumeMount) RadixApplicationComponentBuilder
	WithEnabled(bool) RadixApplicationComponentBuilder
	WithIdentity(*radixv1.Identity) RadixApplicationComponentBuilder
	WithReadOnlyFileSystem(*bool) RadixApplicationComponentBuilder
	WithHorizontalScaling(scaling *radixv1.RadixHorizontalScaling) RadixApplicationComponentBuilder
	WithRuntime(runtime *radixv1.Runtime) RadixApplicationComponentBuilder
	WithNetwork(network *radixv1.Network) RadixApplicationComponentBuilder
	BuildComponent() radixv1.RadixComponent
}

type radixApplicationComponentBuilder struct {
	name                    string
	sourceFolder            string
	dockerfileName          string
	image                   string
	alwaysPullImageOnDeploy *bool
	public                  bool // Deprecated: For backwards compatibility public is still supported, new code should use publicPort instead
	publicPort              string
	monitoringConfig        radixv1.MonitoringConfig
	ports                   []radixv1.ComponentPort
	secrets                 []string
	secretRefs              radixv1.RadixSecretRefs
	ingressConfiguration    []string
	environmentConfig       []RadixEnvironmentConfigBuilder
	variables               radixv1.EnvVarsMap
	resources               radixv1.ResourceRequirements
	node                    radixv1.RadixNode
	authentication          *radixv1.Authentication
	volumeMounts            []radixv1.RadixVolumeMount
	enabled                 *bool
	identity                *radixv1.Identity
	readOnlyFileSystem      *bool
	monitoring              *bool
	imageTagName            string
	horizontalScaling       *radixv1.RadixHorizontalScaling
	runtime                 *radixv1.Runtime
	network                 *radixv1.Network
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

func (rcb *radixApplicationComponentBuilder) WithImageTagName(imageTagName string) RadixApplicationComponentBuilder {
	rcb.imageTagName = imageTagName
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

func (rcb *radixApplicationComponentBuilder) WithSecretRefs(secretRefs radixv1.RadixSecretRefs) RadixApplicationComponentBuilder {
	rcb.secretRefs = secretRefs
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithMonitoringConfig(monitoringConfig radixv1.MonitoringConfig) RadixApplicationComponentBuilder {
	rcb.monitoringConfig = monitoringConfig
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithMonitoring(monitoring *bool) RadixApplicationComponentBuilder {
	rcb.monitoring = monitoring
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) RadixApplicationComponentBuilder {
	rcb.ingressConfiguration = ingressConfiguration
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPort(name string, port int32) RadixApplicationComponentBuilder {
	if rcb.ports == nil {
		rcb.ports = make([]radixv1.ComponentPort, 0)
	}

	rcb.ports = append(rcb.ports, radixv1.ComponentPort{Name: name, Port: port})
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPorts(ports []radixv1.ComponentPort) RadixApplicationComponentBuilder {
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
		rcb.variables = make(radixv1.EnvVarsMap)
	}

	rcb.variables[name] = value
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithNode(node radixv1.RadixNode) RadixApplicationComponentBuilder {
	rcb.node = node
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithAuthentication(authentication *radixv1.Authentication) RadixApplicationComponentBuilder {
	rcb.authentication = authentication
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithVolumeMounts(volumes []radixv1.RadixVolumeMount) RadixApplicationComponentBuilder {
	rcb.volumeMounts = volumes
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnabled(enabled bool) RadixApplicationComponentBuilder {
	rcb.enabled = &enabled
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithIdentity(identity *radixv1.Identity) RadixApplicationComponentBuilder {
	rcb.identity = identity
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithCommonResource(request map[string]string, limit map[string]string) RadixApplicationComponentBuilder {
	rcb.resources = radixv1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithReadOnlyFileSystem(readOnlyFileSystem *bool) RadixApplicationComponentBuilder {
	rcb.readOnlyFileSystem = readOnlyFileSystem
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithHorizontalScaling(scaling *radixv1.RadixHorizontalScaling) RadixApplicationComponentBuilder {
	rcb.horizontalScaling = scaling
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithRuntime(runtime *radixv1.Runtime) RadixApplicationComponentBuilder {
	rcb.runtime = runtime
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithNetwork(network *radixv1.Network) RadixApplicationComponentBuilder {
	rcb.network = network
	return rcb
}

func (rcb *radixApplicationComponentBuilder) BuildComponent() radixv1.RadixComponent {
	var environmentConfig = make([]radixv1.RadixEnvironmentConfig, 0)
	for _, env := range rcb.environmentConfig {
		environmentConfig = append(environmentConfig, env.BuildEnvironmentConfig())
	}

	return radixv1.RadixComponent{
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
		ReadOnlyFileSystem:      rcb.readOnlyFileSystem,
		Monitoring:              rcb.monitoring,
		ImageTagName:            rcb.imageTagName,
		HorizontalScaling:       rcb.horizontalScaling,
		VolumeMounts:            rcb.volumeMounts,
		Runtime:                 rcb.runtime,
		Network:                 rcb.network,
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
