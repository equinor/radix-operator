package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixJobComponentEnvironmentConfigBuilder Handles construction of RA job component environment
type RadixJobComponentEnvironmentConfigBuilder interface {
	WithEnvironment(string) RadixJobComponentEnvironmentConfigBuilder
	WithEnvironmentVariable(string, string) RadixJobComponentEnvironmentConfigBuilder
	WithResource(map[string]string, map[string]string) RadixJobComponentEnvironmentConfigBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) RadixJobComponentEnvironmentConfigBuilder
	WithMonitoring(bool) RadixJobComponentEnvironmentConfigBuilder
	WithImageTagName(string) RadixJobComponentEnvironmentConfigBuilder
	WithNode(node v1.RadixNode) RadixJobComponentEnvironmentConfigBuilder
	WithRunAsNonRoot(bool) RadixJobComponentEnvironmentConfigBuilder
	WithTimeLimitSeconds(*int64) RadixJobComponentEnvironmentConfigBuilder
	BuildEnvironmentConfig() v1.RadixJobComponentEnvironmentConfig
}

type radixJobComponentEnvironmentConfigBuilder struct {
	environment      string
	variables        v1.EnvVarsMap
	resources        v1.ResourceRequirements
	volumeMounts     []v1.RadixVolumeMount
	imageTagName     string
	monitoring       bool
	node             v1.RadixNode
	runAsNonRoot     bool
	timeLimitSeconds *int64
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithTimeLimitSeconds(timeLimitSeconds *int64) RadixJobComponentEnvironmentConfigBuilder {
	ceb.timeLimitSeconds = timeLimitSeconds
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

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithEnvironmentVariable(name, value string) RadixJobComponentEnvironmentConfigBuilder {
	if ceb.variables == nil {
		ceb.variables = make(v1.EnvVarsMap)
	}

	ceb.variables[name] = value
	return ceb
}

func (ceb *radixJobComponentEnvironmentConfigBuilder) WithMonitoring(enabled bool) RadixJobComponentEnvironmentConfigBuilder {
	ceb.monitoring = enabled
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

func (ceb *radixJobComponentEnvironmentConfigBuilder) BuildEnvironmentConfig() v1.RadixJobComponentEnvironmentConfig {
	return v1.RadixJobComponentEnvironmentConfig{
		Environment:      ceb.environment,
		Variables:        ceb.variables,
		Resources:        ceb.resources,
		VolumeMounts:     ceb.volumeMounts,
		Monitoring:       ceb.monitoring,
		ImageTagName:     ceb.imageTagName,
		Node:             ceb.node,
		RunAsNonRoot:     ceb.runAsNonRoot,
		TimeLimitSeconds: ceb.timeLimitSeconds,
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
