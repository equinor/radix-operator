package utils

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// SubPipelineBuilder the sub-pipeline builder
type SubPipelineBuilder interface {
	WithEnvVars(envVars radixv1.EnvVarsMap) SubPipelineBuilder
	WithIdentity(identity *radixv1.Identity) SubPipelineBuilder
	Build() *radixv1.SubPipeline
}

type subPipelineBuilder struct {
	envVars  radixv1.EnvVarsMap
	identity *radixv1.Identity
}

// WithEnvVars sub-pipeline env-vars
func (s *subPipelineBuilder) WithEnvVars(envVars radixv1.EnvVarsMap) SubPipelineBuilder {
	s.envVars = envVars
	return s
}

// WithIdentity sub-pipeline identity
func (s *subPipelineBuilder) WithIdentity(identity *radixv1.Identity) SubPipelineBuilder {
	s.identity = identity
	return s
}

// Build the sub-pipeline
func (s *subPipelineBuilder) Build() *radixv1.SubPipeline {
	return &radixv1.SubPipeline{
		Variables: s.envVars,
		Identity:  s.identity,
	}
}

// NewSubPipelineBuilder instance of the builder
func NewSubPipelineBuilder() SubPipelineBuilder {
	return &subPipelineBuilder{}
}
