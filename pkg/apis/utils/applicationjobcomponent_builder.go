package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixApplicationJobComponentBuilder Handles construction of RA job component
type RadixApplicationJobComponentBuilder interface {
	WithName(string) RadixApplicationJobComponentBuilder
	WithSourceFolder(string) RadixApplicationJobComponentBuilder
	WithDockerfileName(string) RadixApplicationJobComponentBuilder
	WithImage(string) RadixApplicationJobComponentBuilder
	WithImageTagName(imageTagName string) RadixApplicationJobComponentBuilder
	WithPort(string, int32) RadixApplicationJobComponentBuilder
	WithSecrets(...string) RadixApplicationJobComponentBuilder
	WithSecretRefs(v1.RadixSecretRefs) RadixApplicationJobComponentBuilder
	WithMonitoringConfig(v1.MonitoringConfig) RadixApplicationJobComponentBuilder
	WithMonitoring(monitoring *bool) RadixApplicationJobComponentBuilder
	WithEnvironmentConfig(RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder
	WithEnvironmentConfigs(...RadixJobComponentEnvironmentConfigBuilder) RadixApplicationJobComponentBuilder
	WithCommonEnvironmentVariable(string, string) RadixApplicationJobComponentBuilder
	WithCommonResource(map[string]string, map[string]string) RadixApplicationJobComponentBuilder
	WithSchedulerPort(*int32) RadixApplicationJobComponentBuilder
	WithPayloadPath(*string) RadixApplicationJobComponentBuilder
	WithNode(node v1.RadixNode) RadixApplicationJobComponentBuilder
	WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) RadixApplicationJobComponentBuilder
	WithTimeLimitSeconds(*int64) RadixApplicationJobComponentBuilder
	WithBackoffLimit(*int32) RadixApplicationJobComponentBuilder
	WithEnabled(bool) RadixApplicationJobComponentBuilder
	WithIdentity(*v1.Identity) RadixApplicationJobComponentBuilder
	WithNotifications(*v1.Notifications) RadixApplicationJobComponentBuilder
	WithReadOnlyFileSystem(*bool) RadixApplicationJobComponentBuilder
	BuildJobComponent() v1.RadixJobComponent
}

type radixApplicationJobComponentBuilder struct {
	name               string
	sourceFolder       string
	dockerfileName     string
	image              string
	ports              []v1.ComponentPort
	secrets            []string
	secretRefs         v1.RadixSecretRefs
	monitoringConfig   v1.MonitoringConfig
	environmentConfig  []RadixJobComponentEnvironmentConfigBuilder
	variables          v1.EnvVarsMap
	resources          v1.ResourceRequirements
	schedulerPort      *int32
	payloadPath        *string
	node               v1.RadixNode
	volumes            []v1.RadixVolumeMount
	timeLimitSeconds   *int64
	backoffLimit       *int32
	enabled            *bool
	identity           *v1.Identity
	notifications      *v1.Notifications
	readOnlyFileSystem *bool
	monitoring         *bool
	imageTagName       string
}

func (rcb *radixApplicationJobComponentBuilder) WithTimeLimitSeconds(timeLimitSeconds *int64) RadixApplicationJobComponentBuilder {
	rcb.timeLimitSeconds = timeLimitSeconds
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithBackoffLimit(backoffLimit *int32) RadixApplicationJobComponentBuilder {
	rcb.backoffLimit = backoffLimit
	return rcb
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

func (rcb *radixApplicationJobComponentBuilder) WithImageTagName(imageTagName string) RadixApplicationJobComponentBuilder {
	rcb.imageTagName = imageTagName
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithSecrets(secrets ...string) RadixApplicationJobComponentBuilder {
	rcb.secrets = secrets
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) RadixApplicationJobComponentBuilder {
	rcb.secretRefs = secretRefs
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithPort(name string, port int32) RadixApplicationJobComponentBuilder {
	if rcb.ports == nil {
		rcb.ports = make([]v1.ComponentPort, 0)
	}

	rcb.ports = append(rcb.ports, v1.ComponentPort{Name: name, Port: port})
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithPorts(ports []v1.ComponentPort) RadixApplicationJobComponentBuilder {
	for i := range ports {
		rcb.WithPort(ports[i].Name, ports[i].Port)
	}
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithMonitoringConfig(monitoringConfig v1.MonitoringConfig) RadixApplicationJobComponentBuilder {
	rcb.monitoringConfig = monitoringConfig
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithMonitoring(monitoring *bool) RadixApplicationJobComponentBuilder {
	rcb.monitoring = monitoring
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

func (rcb *radixApplicationJobComponentBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) RadixApplicationJobComponentBuilder {
	rcb.volumes = volumeMounts
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithEnabled(enabled bool) RadixApplicationJobComponentBuilder {
	rcb.enabled = &enabled
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithIdentity(identity *v1.Identity) RadixApplicationJobComponentBuilder {
	rcb.identity = identity
	return rcb
}

func (rcb *radixApplicationJobComponentBuilder) WithNotifications(notifications *v1.Notifications) RadixApplicationJobComponentBuilder {
	rcb.notifications = notifications
	return rcb
}
func (rcb *radixApplicationJobComponentBuilder) WithReadOnlyFileSystem(readOnlyFileSystem *bool) RadixApplicationJobComponentBuilder {
	rcb.readOnlyFileSystem = readOnlyFileSystem
	return rcb
}
func (rcb *radixApplicationJobComponentBuilder) BuildJobComponent() v1.RadixJobComponent {
	var environmentConfig = make([]v1.RadixJobComponentEnvironmentConfig, 0)
	for _, env := range rcb.environmentConfig {
		environmentConfig = append(environmentConfig, env.BuildEnvironmentConfig())
	}

	var payload *v1.RadixJobComponentPayload
	if rcb.payloadPath != nil {
		payload = &v1.RadixJobComponentPayload{Path: *rcb.payloadPath}
	}

	return v1.RadixJobComponent{
		Name:               rcb.name,
		SourceFolder:       rcb.sourceFolder,
		DockerfileName:     rcb.dockerfileName,
		Image:              rcb.image,
		Ports:              rcb.ports,
		Secrets:            rcb.secrets,
		SecretRefs:         rcb.secretRefs,
		MonitoringConfig:   rcb.monitoringConfig,
		EnvironmentConfig:  environmentConfig,
		Variables:          rcb.variables,
		Resources:          rcb.resources,
		SchedulerPort:      rcb.schedulerPort,
		Payload:            payload,
		Node:               rcb.node,
		TimeLimitSeconds:   rcb.timeLimitSeconds,
		BackoffLimit:       rcb.backoffLimit,
		Enabled:            rcb.enabled,
		Identity:           rcb.identity,
		Notifications:      rcb.notifications,
		ReadOnlyFileSystem: rcb.readOnlyFileSystem,
		Monitoring:         rcb.monitoring,
		ImageTagName:       rcb.imageTagName,
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
