package utils

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixEnvironmentConfigBuilder Handles construction of RA component environment
type RadixEnvironmentConfigBuilder interface {
	WithEnvironment(string) RadixEnvironmentConfigBuilder
	WithSourceFolder(string) RadixEnvironmentConfigBuilder
	WithDockerfileName(string) RadixEnvironmentConfigBuilder
	WithImage(string) RadixEnvironmentConfigBuilder
	WithReplicas(*int) RadixEnvironmentConfigBuilder
	WithEnvironmentVariable(string, string) RadixEnvironmentConfigBuilder
	WithResource(map[string]string, map[string]string) RadixEnvironmentConfigBuilder
	WithVolumeMounts([]radixv1.RadixVolumeMount) RadixEnvironmentConfigBuilder
	BuildEnvironmentConfig() radixv1.RadixEnvironmentConfig
	WithAlwaysPullImageOnDeploy(bool) RadixEnvironmentConfigBuilder
	WithMonitoring(monitoring *bool) RadixEnvironmentConfigBuilder
	WithNode(radixv1.RadixNode) RadixEnvironmentConfigBuilder
	WithAuthentication(*radixv1.Authentication) RadixEnvironmentConfigBuilder
	WithSecretRefs(radixv1.RadixSecretRefs) RadixEnvironmentConfigBuilder
	WithEnabled(bool) RadixEnvironmentConfigBuilder
	WithIdentity(*radixv1.Identity) RadixEnvironmentConfigBuilder
	WithImageTagName(string) RadixEnvironmentConfigBuilder
	WithHorizontalScaling(scaling *radixv1.RadixHorizontalScaling) RadixEnvironmentConfigBuilder
	WithReadOnlyFileSystem(*bool) RadixEnvironmentConfigBuilder
	WithRuntime(*radixv1.Runtime) RadixEnvironmentConfigBuilder
	WithNetwork(*radixv1.Network) RadixEnvironmentConfigBuilder
}

type radixEnvironmentConfigBuilder struct {
	environment             string
	sourceFolder            string
	dockerfileName          string
	image                   string
	variables               radixv1.EnvVarsMap
	replicas                *int
	resources               radixv1.ResourceRequirements
	alwaysPullImageOnDeploy *bool
	volumeMounts            []radixv1.RadixVolumeMount
	monitoring              *bool
	node                    radixv1.RadixNode
	secretRefs              radixv1.RadixSecretRefs
	authentication          *radixv1.Authentication
	enabled                 *bool
	identity                *radixv1.Identity
	imageTagName            string
	horizontalScaling       *radixv1.RadixHorizontalScaling
	readOnlyFileSystem      *bool
	runtime                 *radixv1.Runtime
	network                 *radixv1.Network
}

func (ceb *radixEnvironmentConfigBuilder) WithHorizontalScaling(scaling *radixv1.RadixHorizontalScaling) RadixEnvironmentConfigBuilder {
	ceb.horizontalScaling = scaling
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithResource(request map[string]string, limit map[string]string) RadixEnvironmentConfigBuilder {
	ceb.resources = radixv1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithVolumeMounts(volumeMounts []radixv1.RadixVolumeMount) RadixEnvironmentConfigBuilder {
	ceb.volumeMounts = volumeMounts
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithEnvironment(environment string) RadixEnvironmentConfigBuilder {
	ceb.environment = environment
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithSourceFolder(sourceFolder string) RadixEnvironmentConfigBuilder {
	ceb.sourceFolder = sourceFolder
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithDockerfileName(dockerfileName string) RadixEnvironmentConfigBuilder {
	ceb.dockerfileName = dockerfileName
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithImage(image string) RadixEnvironmentConfigBuilder {
	ceb.image = image
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithReplicas(replicas *int) RadixEnvironmentConfigBuilder {
	ceb.replicas = replicas
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithEnvironmentVariable(name, value string) RadixEnvironmentConfigBuilder {
	if ceb.variables == nil {
		ceb.variables = make(radixv1.EnvVarsMap)
	}

	ceb.variables[name] = value
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithAlwaysPullImageOnDeploy(val bool) RadixEnvironmentConfigBuilder {
	ceb.alwaysPullImageOnDeploy = &val
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithMonitoring(monitoring *bool) RadixEnvironmentConfigBuilder {
	ceb.monitoring = monitoring
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithNode(node radixv1.RadixNode) RadixEnvironmentConfigBuilder {
	ceb.node = node
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithAuthentication(authentication *radixv1.Authentication) RadixEnvironmentConfigBuilder {
	ceb.authentication = authentication
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithSecretRefs(secretRefs radixv1.RadixSecretRefs) RadixEnvironmentConfigBuilder {
	ceb.secretRefs = secretRefs
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithEnabled(enabled bool) RadixEnvironmentConfigBuilder {
	ceb.enabled = &enabled
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithIdentity(identity *radixv1.Identity) RadixEnvironmentConfigBuilder {
	ceb.identity = identity
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithImageTagName(imageTagName string) RadixEnvironmentConfigBuilder {
	ceb.imageTagName = imageTagName
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithReadOnlyFileSystem(readOnlyFileSystem *bool) RadixEnvironmentConfigBuilder {
	ceb.readOnlyFileSystem = readOnlyFileSystem
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithRuntime(runtime *radixv1.Runtime) RadixEnvironmentConfigBuilder {
	ceb.runtime = runtime
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) WithNetwork(network *radixv1.Network) RadixEnvironmentConfigBuilder {
	ceb.network = network
	return ceb
}

func (ceb *radixEnvironmentConfigBuilder) BuildEnvironmentConfig() radixv1.RadixEnvironmentConfig {
	return radixv1.RadixEnvironmentConfig{
		Environment:             ceb.environment,
		SourceFolder:            ceb.sourceFolder,
		DockerfileName:          ceb.dockerfileName,
		Image:                   ceb.image,
		Variables:               ceb.variables,
		Replicas:                ceb.replicas,
		Resources:               ceb.resources,
		VolumeMounts:            ceb.volumeMounts,
		Node:                    ceb.node,
		SecretRefs:              ceb.secretRefs,
		Monitoring:              ceb.monitoring,
		AlwaysPullImageOnDeploy: ceb.alwaysPullImageOnDeploy,
		Authentication:          ceb.authentication,
		Enabled:                 ceb.enabled,
		Identity:                ceb.identity,
		ImageTagName:            ceb.imageTagName,
		HorizontalScaling:       ceb.horizontalScaling,
		ReadOnlyFileSystem:      ceb.readOnlyFileSystem,
		Runtime:                 ceb.runtime,
		Network:                 ceb.network,
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
