package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixJobComponentEnvironmentConfigBuilder Handles construction of RA job component environment
type RadixJobComponentEnvironmentConfigBuilder interface {
	WithEnvironment(string) RadixJobComponentEnvironmentConfigBuilder
	WithSourceFolder(string) RadixJobComponentEnvironmentConfigBuilder
	WithDockerfileName(string) RadixJobComponentEnvironmentConfigBuilder
	WithImage(string) RadixJobComponentEnvironmentConfigBuilder
	WithEnvironmentVariable(string, string) RadixJobComponentEnvironmentConfigBuilder
	WithResource(map[string]string, map[string]string) RadixJobComponentEnvironmentConfigBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) RadixJobComponentEnvironmentConfigBuilder
	WithMonitoring(monitoring *bool) RadixJobComponentEnvironmentConfigBuilder
	WithImageTagName(string) RadixJobComponentEnvironmentConfigBuilder
	WithNode(v1.RadixNode) RadixJobComponentEnvironmentConfigBuilder
	WithRunAsNonRoot(bool) RadixJobComponentEnvironmentConfigBuilder
	WithTimeLimitSeconds(*int64) RadixJobComponentEnvironmentConfigBuilder
	WithBackoffLimit(*int32) RadixJobComponentEnvironmentConfigBuilder
	WithSecretRefs(v1.RadixSecretRefs) RadixJobComponentEnvironmentConfigBuilder
	WithEnabled(bool) RadixJobComponentEnvironmentConfigBuilder
	WithIdentity(*v1.Identity) RadixJobComponentEnvironmentConfigBuilder
	WithNotifications(*v1.Notifications) RadixJobComponentEnvironmentConfigBuilder
	WithReadOnlyFileSystem(*bool) RadixJobComponentEnvironmentConfigBuilder
	WithRuntime(*v1.Runtime) RadixJobComponentEnvironmentConfigBuilder
	WithCommand(strings []string) RadixJobComponentEnvironmentConfigBuilder
	WithArgs(strings []string) RadixJobComponentEnvironmentConfigBuilder
	WithFailurePolicy(*v1.RadixJobComponentFailurePolicy) RadixJobComponentEnvironmentConfigBuilder
	BuildEnvironmentConfig() v1.RadixJobComponentEnvironmentConfig
}

type radixJobComponentEnvironmentConfigBuilder struct {
	environment        string
	sourceFolder       string
	dockerfileName     string
	image              string
	variables          v1.EnvVarsMap
	resources          v1.ResourceRequirements
	volumeMounts       []v1.RadixVolumeMount
	imageTagName       string
	monitoring         *bool
	node               v1.RadixNode
	runAsNonRoot       bool
	secretRefs         v1.RadixSecretRefs
	timeLimitSeconds   *int64
	backoffLimit       *int32
	enabled            *bool
	identity           *v1.Identity
	notifications      *v1.Notifications
	readOnlyFileSystem *bool
	runtime            *v1.Runtime
	failurePolicy      *v1.RadixJobComponentFailurePolicy
	command            *[]string
	args               *[]string
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithTimeLimitSeconds(timeLimitSeconds *int64) RadixJobComponentEnvironmentConfigBuilder {
	ceb.timeLimitSeconds = timeLimitSeconds
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithBackoffLimit(backoffLimit *int32) RadixJobComponentEnvironmentConfigBuilder {
	ceb.backoffLimit = backoffLimit
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithResource(request map[string]string, limit map[string]string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) RadixJobComponentEnvironmentConfigBuilder {
	ceb.volumeMounts = volumeMounts
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithEnvironment(environment string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.environment = environment
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithSourceFolder(sourceFolder string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.sourceFolder = sourceFolder
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithDockerfileName(dockerfileName string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.dockerfileName = dockerfileName
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithImage(image string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.image = image
	return ceb
}
func (ceb *radixJobComponentEnvironmentConfigBuilder) WithEnvironmentVariable(name, value string) RadixJobComponentEnvironmentConfigBuilder {
	if ceb.variables == nil {
		ceb.variables = make(v1.EnvVarsMap)
	}

	ceb.variables[name] = value
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithMonitoring(monitoring *bool) RadixJobComponentEnvironmentConfigBuilder {
	ceb.monitoring = monitoring
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithImageTagName(imageTagName string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.imageTagName = imageTagName
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithNode(node v1.RadixNode) RadixJobComponentEnvironmentConfigBuilder {
	ceb.node = node
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithRunAsNonRoot(runAsNonRoot bool) RadixJobComponentEnvironmentConfigBuilder {
	ceb.runAsNonRoot = runAsNonRoot
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) RadixJobComponentEnvironmentConfigBuilder {
	ceb.secretRefs = secretRefs
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithEnabled(enabled bool) RadixJobComponentEnvironmentConfigBuilder {
	ceb.enabled = &enabled
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithIdentity(identity *v1.Identity) RadixJobComponentEnvironmentConfigBuilder {
	ceb.identity = identity
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithNotifications(notifications *v1.Notifications) RadixJobComponentEnvironmentConfigBuilder {
	ceb.notifications = notifications
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithReadOnlyFileSystem(readOnlyFileSystem *bool) RadixJobComponentEnvironmentConfigBuilder {
	ceb.readOnlyFileSystem = readOnlyFileSystem
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithRuntime(runtime *v1.Runtime) RadixJobComponentEnvironmentConfigBuilder {
	ceb.runtime = runtime
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithCommand(command []string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.command = &command
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithArgs(args []string) RadixJobComponentEnvironmentConfigBuilder {
	ceb.args = &args
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithFailurePolicy(failurePolicy *v1.RadixJobComponentFailurePolicy) RadixJobComponentEnvironmentConfigBuilder {
	ceb.failurePolicy = failurePolicy
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) BuildEnvironmentConfig() v1.RadixJobComponentEnvironmentConfig {
	return v1.RadixJobComponentEnvironmentConfig{
		Environment:        ceb.environment,
		SourceFolder:       ceb.sourceFolder,
		DockerfileName:     ceb.dockerfileName,
		Image:              ceb.image,
		Variables:          ceb.variables,
		Resources:          ceb.resources,
		VolumeMounts:       ceb.volumeMounts,
		Monitoring:         ceb.monitoring,
		ImageTagName:       ceb.imageTagName,
		Node:               ceb.node,
		SecretRefs:         ceb.secretRefs,
		TimeLimitSeconds:   ceb.timeLimitSeconds,
		BackoffLimit:       ceb.backoffLimit,
		Enabled:            ceb.enabled,
		Identity:           ceb.identity,
		Notifications:      ceb.notifications,
		ReadOnlyFileSystem: ceb.readOnlyFileSystem,
		Runtime:            ceb.runtime,
		FailurePolicy:      ceb.failurePolicy,
		Command:            ceb.command,
		Args:               ceb.args,
	}
}

// NewJobComponentEnvironmentBuilder Constructor for job component environment builder
func NewJobComponentEnvironmentBuilder() RadixJobComponentEnvironmentConfigBuilder {
	return &radixJobComponentEnvironmentConfigBuilder{}
}

// AJobComponentEnvironmentConfig Constructor for job component environment builder containing test data
func AJobComponentEnvironmentConfig() RadixJobComponentEnvironmentConfigBuilder {
	return &radixJobComponentEnvironmentConfigBuilder{
		environment: "app",
	}
}
