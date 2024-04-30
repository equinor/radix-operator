package utils

import (
	commonUtils "github.com/equinor/radix-common/utils"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ApplicationEnvironmentBuilder interface
type ApplicationEnvironmentBuilder interface {
	WithName(name string) ApplicationEnvironmentBuilder
	WithBuildFrom(branch string) ApplicationEnvironmentBuilder
	WithEnvVars(envVars radixv1.EnvVarsMap) ApplicationEnvironmentBuilder
	WithSubPipeline(subPipelineBuilder SubPipelineBuilder) ApplicationEnvironmentBuilder
	Build() radixv1.Environment
}

type applicationEnvironmentBuilder struct {
	name               string
	branch             string
	envVars            radixv1.EnvVarsMap
	subPipelineBuilder SubPipelineBuilder
}

// NewApplicationEnvironmentBuilder new instance of the ApplicationEnvironmentBuilder
func NewApplicationEnvironmentBuilder() ApplicationEnvironmentBuilder {
	return &applicationEnvironmentBuilder{}
}

// WithName a name of an application environment
func (b *applicationEnvironmentBuilder) WithName(name string) ApplicationEnvironmentBuilder {
	b.name = name
	return b
}

// WithBuildFrom an application environment build from branch
func (b *applicationEnvironmentBuilder) WithBuildFrom(branch string) ApplicationEnvironmentBuilder {
	b.branch = branch
	return b
}

// WithEnvVars build env-vars
func (b *applicationEnvironmentBuilder) WithEnvVars(envVars radixv1.EnvVarsMap) ApplicationEnvironmentBuilder {
	b.envVars = envVars
	return b
}

// WithSubPipeline sub-pipeline config
func (b *applicationEnvironmentBuilder) WithSubPipeline(subPipelineBuilder SubPipelineBuilder) ApplicationEnvironmentBuilder {
	b.subPipelineBuilder = subPipelineBuilder
	return b
}

// Build an application environment
func (b *applicationEnvironmentBuilder) Build() radixv1.Environment {
	environment := radixv1.Environment{
		Name: b.name,
		Build: radixv1.EnvBuild{
			From:      b.branch,
			Variables: b.envVars,
		},
	}
	if !commonUtils.IsNil(b.subPipelineBuilder) {
		environment.SubPipeline = b.subPipelineBuilder.Build()
	}
	return environment
}
