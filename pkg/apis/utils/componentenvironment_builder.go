package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixEnvironmentConfigBuilder Handles construction of RA component environment
type RadixEnvironmentConfigBuilder interface {
	WithEnvironment(string) RadixEnvironmentConfigBuilder
	WithReplicas(*int) RadixEnvironmentConfigBuilder
	WithEnvironmentVariable(string, string) RadixEnvironmentConfigBuilder
	WithResource(map[string]string, map[string]string) RadixEnvironmentConfigBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) RadixEnvironmentConfigBuilder
	BuildEnvironmentConfig() v1.RadixEnvironmentConfig
	WithAlwaysPullImageOnDeploy(bool) RadixEnvironmentConfigBuilder
	WithMonitoring(bool) RadixEnvironmentConfigBuilder
	WithNode(node v1.RadixNode) RadixEnvironmentConfigBuilder
	WithAuthentication(authentication *v1.Authentication) RadixEnvironmentConfigBuilder
	WithSecretRefs(secretRefs v1.RadixSecretRefs) RadixEnvironmentConfigBuilder
}

type radixEnvironmentConfigBuilder struct {
	environment             string
	variables               v1.EnvVarsMap
	replicas                *int
	resources               v1.ResourceRequirements
	alwaysPullImageOnDeploy *bool
	volumeMounts            []v1.RadixVolumeMount
	monitoring              bool
	node                    v1.RadixNode
	secretRefs              v1.RadixSecretRefs
	authentication          *v1.Authentication
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
	if ceb.variables == nil {
		ceb.variables = make(v1.EnvVarsMap)
	}

	ceb.variables[name] = value
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithAlwaysPullImageOnDeploy(val bool) RadixEnvironmentConfigBuilder {
	ceb.alwaysPullImageOnDeploy = &val
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithMonitoring(monitoring bool) RadixEnvironmentConfigBuilder {
	ceb.monitoring = monitoring
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithNode(node v1.RadixNode) RadixEnvironmentConfigBuilder {
	ceb.node = node
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithAuthentication(authentication *v1.Authentication) RadixEnvironmentConfigBuilder {
	ceb.authentication = authentication
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) RadixEnvironmentConfigBuilder {
	ceb.secretRefs = secretRefs
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) BuildEnvironmentConfig() v1.RadixEnvironmentConfig {
	return v1.RadixEnvironmentConfig{
		Environment:             ceb.environment,
		Variables:               ceb.variables,
		Replicas:                ceb.replicas,
		Resources:               ceb.resources,
		VolumeMounts:            ceb.volumeMounts,
		Node:                    ceb.node,
		SecretRefs:              ceb.secretRefs,
		Monitoring:              ceb.monitoring,
		AlwaysPullImageOnDeploy: ceb.alwaysPullImageOnDeploy,
		Authentication:          ceb.authentication,
	}
}

// NewComponentEnvironmentBuilder Constructor for component environment builder
func NewComponentEnvironmentBuilder() RadixEnvironmentConfigBuilder {
	return &radixEnvironmentConfigBuilder{}
}

// AnEnvironmentConfig Constructor for component environment builder containing test data
func AnEnvironmentConfig() RadixEnvironmentConfigBuilder {
	return &radixEnvironmentConfigBuilder{
		environment: "app",
	}
}
