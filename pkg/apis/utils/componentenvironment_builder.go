package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixEnvironmentConfigBuilder Handles construction of RA component environment
type RadixEnvironmentConfigBuilder interface {
	WithEnvironment(string) RadixEnvironmentConfigBuilder
	WithReplicas(*int) RadixEnvironmentConfigBuilder
	WithEnvironmentVariable(string, string) RadixEnvironmentConfigBuilder
	WithResource(map[string]string, map[string]string) RadixEnvironmentConfigBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) RadixEnvironmentConfigBuilder
	BuildEnvironmentConfig() v1.RadixEnvironmentConfig
	WithAlwaysPullImageOnDeploy(bool) RadixEnvironmentConfigBuilder
	WithNilVariablesMap() RadixEnvironmentConfigBuilder
	WithRunAsNonRoot(bool) RadixEnvironmentConfigBuilder
}

type radixEnvironmentConfigBuilder struct {
	environment             string
	variables               v1.EnvVarsMap
	replicas                *int
	ports                   map[string]int32
	secrets                 []string
	resources               v1.ResourceRequirements
	alwaysPullImageOnDeploy *bool
	volumeMounts            []v1.RadixVolumeMount
	runAsNonRoot            bool
}

func (ceb *radixEnvironmentConfigBuilder) WithResource(request map[string]string, limit map[string]string) RadixEnvironmentConfigBuilder {
	ceb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) RadixEnvironmentConfigBuilder {
	ceb.volumeMounts = volumeMounts
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithEnvironment(environment string) RadixEnvironmentConfigBuilder {
	ceb.environment = environment
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithReplicas(replicas *int) RadixEnvironmentConfigBuilder {
	ceb.replicas = replicas
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithEnvironmentVariable(name, value string) RadixEnvironmentConfigBuilder {
	ceb.variables[name] = value
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithAlwaysPullImageOnDeploy(val bool) RadixEnvironmentConfigBuilder {
	ceb.alwaysPullImageOnDeploy = &val
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithNilVariablesMap() RadixEnvironmentConfigBuilder {
	ceb.variables = nil
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithRunAsNonRoot(runAsNonRoot bool) RadixEnvironmentConfigBuilder {
	ceb.runAsNonRoot = runAsNonRoot
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) BuildEnvironmentConfig() v1.RadixEnvironmentConfig {
	return v1.RadixEnvironmentConfig{
		Environment:             ceb.environment,
		Variables:               ceb.variables,
		Replicas:                ceb.replicas,
		Resources:               ceb.resources,
		VolumeMounts:            ceb.volumeMounts,
		AlwaysPullImageOnDeploy: ceb.alwaysPullImageOnDeploy,
		RunAsNonRoot:            ceb.runAsNonRoot,
	}
}

// NewComponentEnvironmentBuilder Constructor for component environment builder
func NewComponentEnvironmentBuilder() RadixEnvironmentConfigBuilder {
	return &radixEnvironmentConfigBuilder{
		variables: make(map[string]string),
	}
}

// AnEnvironmentConfig Constructor for component environment builder containing test data
func AnEnvironmentConfig() RadixEnvironmentConfigBuilder {
	return &radixEnvironmentConfigBuilder{
		environment: "app",
		variables:   make(map[string]string),
	}
}
