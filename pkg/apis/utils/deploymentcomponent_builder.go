package utils

import (
	"github.com/equinor/radix-common/utils/pointers"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeployComponentBuilder Handles construction of RD component
type DeployComponentBuilder interface {
	WithName(string) DeployComponentBuilder
	WithImage(string) DeployComponentBuilder
	WithPort(string, int32) DeployComponentBuilder
	WithPorts([]v1.ComponentPort) DeployComponentBuilder
	WithEnvironmentVariable(string, string) DeployComponentBuilder
	WithEnvironmentVariables(map[string]string) DeployComponentBuilder
	// Deprecated: For backwards compatibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublic(bool) DeployComponentBuilder
	WithPublicPort(string) DeployComponentBuilder
	WithMonitoring(bool) DeployComponentBuilder
	WithMonitoringConfig(v1.MonitoringConfig) DeployComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) DeployComponentBuilder
	WithReplicas(*int) DeployComponentBuilder
	WithReplicasOverride(*int) DeployComponentBuilder
	WithResourceRequestsOnly(map[string]string) DeployComponentBuilder
	WithResource(map[string]string, map[string]string) DeployComponentBuilder
	WithVolumeMounts(...v1.RadixVolumeMount) DeployComponentBuilder
	WithHealthChecks(startupProbe, readynessProbe, livenessProbe *v1.RadixProbe) DeployComponentBuilder
	WithNodeGpu(gpu string) DeployComponentBuilder
	WithNodeGpuCount(gpuCount string) DeployComponentBuilder
	WithNodeType(nodeType string) DeployComponentBuilder
	WithIngressConfiguration(...string) DeployComponentBuilder
	WithSecrets([]string) DeployComponentBuilder
	WithSecretRefs(v1.RadixSecretRefs) DeployComponentBuilder
	WithDNSAppAlias(bool) DeployComponentBuilder
	// Deprecated: For backwards compatibility WithDNSExternalAliases is still supported, new code should use WithPublicPort instead
	WithDNSExternalAliases(...string) DeployComponentBuilder
	WithExternalDNS(...v1.RadixDeployExternalDNS) DeployComponentBuilder
	WithHorizontalScaling(scaling *v1.RadixHorizontalScaling) DeployComponentBuilder
	WithRunAsNonRoot(bool) DeployComponentBuilder
	WithAuthentication(*v1.Authentication) DeployComponentBuilder
	WithIdentity(*v1.Identity) DeployComponentBuilder
	WithReadOnlyFileSystem(*bool) DeployComponentBuilder
	WithRuntime(*v1.Runtime) DeployComponentBuilder
	BuildComponent() v1.RadixDeployComponent
}

type deployComponentBuilder struct {
	name                 string
	runAsNonRoot         bool
	image                string
	ports                []v1.ComponentPort
	environmentVariables map[string]string
	// Deprecated: For backwards comptibility public is still supported, new code should use publicPort instead
	public                  bool
	publicPort              string
	monitoring              bool
	monitoringConfig        v1.MonitoringConfig
	replicas                *int
	replicasOverride        *int
	alwaysPullImageOnDeploy bool
	ingressConfiguration    []string
	secrets                 []string
	secretRefs              v1.RadixSecretRefs
	dnsAppAlias             bool
	// Deprecated: For backwards compatibility externalAppAlias is still supported, new code should use externalDNS instead
	externalAppAlias  []string
	externalDNS       []v1.RadixDeployExternalDNS
	resources         v1.ResourceRequirements
	horizontalScaling *v1.RadixHorizontalScaling
	volumeMounts      []v1.RadixVolumeMount
	// Deprecated: use nodeType instead.
	node               v1.RadixNode
	nodeType           *string
	authentication     *v1.Authentication
	identity           *v1.Identity
	readOnlyFileSystem *bool
	runtime            *v1.Runtime
	healtChecks        *v1.RadixHealthChecks
}

func (dcb *deployComponentBuilder) WithVolumeMounts(volumeMounts ...v1.RadixVolumeMount) DeployComponentBuilder {
	dcb.volumeMounts = volumeMounts
	return dcb
}

func (dcb *deployComponentBuilder) WithHealthChecks(startupProbe, readynessProbe, livenessProbe *v1.RadixProbe) DeployComponentBuilder {
	dcb.healtChecks = &v1.RadixHealthChecks{
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readynessProbe,
		StartupProbe:   startupProbe,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithNodeGpu(gpu string) DeployComponentBuilder {
	dcb.node.Gpu = gpu
	return dcb
}

func (dcb *deployComponentBuilder) WithNodeGpuCount(gpuCount string) DeployComponentBuilder {
	dcb.node.GpuCount = gpuCount
	return dcb
}

func (dcb *deployComponentBuilder) WithNodeType(nodeType string) DeployComponentBuilder {
	dcb.nodeType = pointers.Ptr(nodeType)
	return dcb
}

func (dcb *deployComponentBuilder) WithResourceRequestsOnly(request map[string]string) DeployComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Requests: request,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithResource(request map[string]string, limit map[string]string) DeployComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithName(name string) DeployComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployComponentBuilder) WithAlwaysPullImageOnDeploy(val bool) DeployComponentBuilder {
	dcb.alwaysPullImageOnDeploy = val
	return dcb
}

func (dcb *deployComponentBuilder) WithDNSAppAlias(createDNSAppAlias bool) DeployComponentBuilder {
	dcb.dnsAppAlias = createDNSAppAlias
	return dcb
}

// Deprecated: For backwards compatibility it is still supported, new code should use WithExternalDNS instead
func (dcb *deployComponentBuilder) WithDNSExternalAliases(alias ...string) DeployComponentBuilder {
	dcb.externalAppAlias = alias
	return dcb
}

func (dcb *deployComponentBuilder) WithExternalDNS(externalDNS ...v1.RadixDeployExternalDNS) DeployComponentBuilder {
	dcb.externalDNS = externalDNS
	return dcb
}

func (dcb *deployComponentBuilder) WithImage(image string) DeployComponentBuilder {
	dcb.image = image
	return dcb
}

func (dcb *deployComponentBuilder) WithPort(name string, port int32) DeployComponentBuilder {
	if dcb.ports == nil {
		dcb.ports = make([]v1.ComponentPort, 0)
	}

	dcb.ports = append(dcb.ports, v1.ComponentPort{Name: name, Port: port})
	return dcb
}

func (dcb *deployComponentBuilder) WithPorts(ports []v1.ComponentPort) DeployComponentBuilder {
	for i := range ports {
		dcb.WithPort(ports[i].Name, ports[i].Port)
	}
	return dcb
}

// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
func (dcb *deployComponentBuilder) WithPublic(public bool) DeployComponentBuilder {
	dcb.public = public
	return dcb
}

func (dcb *deployComponentBuilder) WithPublicPort(publicPort string) DeployComponentBuilder {
	dcb.publicPort = publicPort
	return dcb
}

func (dcb *deployComponentBuilder) WithMonitoring(monitoring bool) DeployComponentBuilder {
	dcb.monitoring = monitoring
	return dcb
}

func (dcb *deployComponentBuilder) WithMonitoringConfig(monitoringConfig v1.MonitoringConfig) DeployComponentBuilder {
	dcb.monitoringConfig = monitoringConfig
	return dcb
}

func (dcb *deployComponentBuilder) WithReplicas(replicas *int) DeployComponentBuilder {
	dcb.replicas = replicas
	return dcb
}
func (dcb *deployComponentBuilder) WithReplicasOverride(replicas *int) DeployComponentBuilder {
	dcb.replicasOverride = replicas
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariable(name string, value string) DeployComponentBuilder {
	if dcb.environmentVariables == nil {
		dcb.environmentVariables = make(map[string]string)
	}

	dcb.environmentVariables[name] = value
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariables(environmentVariables map[string]string) DeployComponentBuilder {
	dcb.environmentVariables = environmentVariables
	return dcb
}

func (dcb *deployComponentBuilder) WithSecrets(secrets []string) DeployComponentBuilder {
	dcb.secrets = secrets
	return dcb
}

func (dcb *deployComponentBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) DeployComponentBuilder {
	dcb.secretRefs = secretRefs
	return dcb
}

func (dcb *deployComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) DeployComponentBuilder {
	dcb.ingressConfiguration = ingressConfiguration
	return dcb
}

func (dcb *deployComponentBuilder) WithHorizontalScaling(scaling *v1.RadixHorizontalScaling) DeployComponentBuilder {
	dcb.horizontalScaling = scaling
	return dcb
}

func (dcb *deployComponentBuilder) WithRunAsNonRoot(runAsNonRoot bool) DeployComponentBuilder {
	dcb.runAsNonRoot = runAsNonRoot
	return dcb
}

func (dcb *deployComponentBuilder) WithAuthentication(authentication *v1.Authentication) DeployComponentBuilder {
	dcb.authentication = authentication
	return dcb
}

func (dcb *deployComponentBuilder) WithIdentity(identity *v1.Identity) DeployComponentBuilder {
	dcb.identity = identity
	return dcb
}

func (dcb *deployComponentBuilder) WithReadOnlyFileSystem(readOnlyFileSystem *bool) DeployComponentBuilder {
	dcb.readOnlyFileSystem = readOnlyFileSystem
	return dcb
}

func (dcb *deployComponentBuilder) WithRuntime(runtime *v1.Runtime) DeployComponentBuilder {
	dcb.runtime = runtime
	return dcb
}

func (dcb *deployComponentBuilder) BuildComponent() v1.RadixDeployComponent {
	return v1.RadixDeployComponent{
		Image:                   dcb.image,
		Name:                    dcb.name,
		Ports:                   dcb.ports,
		Public:                  dcb.public,
		PublicPort:              dcb.publicPort,
		Monitoring:              dcb.monitoring,
		MonitoringConfig:        dcb.monitoringConfig,
		Replicas:                dcb.replicas,
		ReplicasOverride:        dcb.replicasOverride,
		Secrets:                 dcb.secrets,
		SecretRefs:              dcb.secretRefs,
		IngressConfiguration:    dcb.ingressConfiguration,
		EnvironmentVariables:    dcb.environmentVariables,
		DNSAppAlias:             dcb.dnsAppAlias,
		DNSExternalAlias:        dcb.externalAppAlias,
		ExternalDNS:             dcb.externalDNS,
		Resources:               dcb.resources,
		HealthChecks:            dcb.healtChecks,
		HorizontalScaling:       dcb.horizontalScaling,
		VolumeMounts:            dcb.volumeMounts,
		AlwaysPullImageOnDeploy: dcb.alwaysPullImageOnDeploy,
		Node:                    dcb.node,
		NodeType:                dcb.nodeType,
		Authentication:          dcb.authentication,
		Identity:                dcb.identity,
		ReadOnlyFileSystem:      dcb.readOnlyFileSystem,
		Runtime:                 dcb.runtime,
	}
}

// NewDeployComponentBuilder Constructor for component builder
func NewDeployComponentBuilder() DeployComponentBuilder {
	return &deployComponentBuilder{}
}
