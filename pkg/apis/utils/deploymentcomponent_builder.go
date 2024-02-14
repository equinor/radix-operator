package utils

import (
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
	WithResourceRequestsOnly(map[string]string) DeployComponentBuilder
	WithResource(map[string]string, map[string]string) DeployComponentBuilder
	WithVolumeMounts(...v1.RadixVolumeMount) DeployComponentBuilder
	WithNodeGpu(gpu string) DeployComponentBuilder
	WithNodeGpuCount(gpuCount string) DeployComponentBuilder
	WithIngressConfiguration(...string) DeployComponentBuilder
	WithSecrets([]string) DeployComponentBuilder
	WithSecretRefs(v1.RadixSecretRefs) DeployComponentBuilder
	WithDNSAppAlias(bool) DeployComponentBuilder
	// Deprecated: For backwards compatibility WithDNSExternalAliases is still supported, new code should use WithPublicPort instead
	WithDNSExternalAliases(...string) DeployComponentBuilder
	WithExternalDNS(...v1.RadixDeployExternalDNS) DeployComponentBuilder
	WithHorizontalScaling(minReplicas *int32, maxReplicas int32, cpu *int32, memory *int32) DeployComponentBuilder
	WithRunAsNonRoot(bool) DeployComponentBuilder
	WithAuthentication(*v1.Authentication) DeployComponentBuilder
	WithIdentity(*v1.Identity) DeployComponentBuilder
	WithReadOnlyFileSystem(*bool) DeployComponentBuilder
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
	alwaysPullImageOnDeploy bool
	ingressConfiguration    []string
	secrets                 []string
	secretRefs              v1.RadixSecretRefs
	dnsappalias             bool
	// Deprecated: For backwards comptibility externalAppAlias is still supported, new code should use publicPort instead
	externalAppAlias   []string
	externalDNS        []v1.RadixDeployExternalDNS
	resources          v1.ResourceRequirements
	horizontalScaling  *v1.RadixHorizontalScaling
	volumeMounts       []v1.RadixVolumeMount
	node               v1.RadixNode
	authentication     *v1.Authentication
	identity           *v1.Identity
	readOnlyFileSystem *bool
}

func (dcb *deployComponentBuilder) WithVolumeMounts(volumeMounts ...v1.RadixVolumeMount) DeployComponentBuilder {
	dcb.volumeMounts = volumeMounts
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
	dcb.dnsappalias = createDNSAppAlias
	return dcb
}

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

func (dcb *deployComponentBuilder) WithHorizontalScaling(minReplicas *int32, maxReplicas int32, cpu *int32, memory *int32) DeployComponentBuilder {
	radixHorizontalScalingResources := &v1.RadixHorizontalScalingResources{}

	// if memory is nil, then memory is omitted while cpu is set to provided value
	if memory == nil {
		radixHorizontalScalingResources.Cpu = &v1.RadixHorizontalScalingResource{
			AverageUtilization: cpu,
		}
	}

	// if cpu and memory are non-nil, then memory and cpu are set to provided values
	if memory != nil {
		radixHorizontalScalingResources.Memory = &v1.RadixHorizontalScalingResource{
			AverageUtilization: memory,
		}
		if cpu != nil {
			radixHorizontalScalingResources.Cpu = &v1.RadixHorizontalScalingResource{
				AverageUtilization: cpu,
			}
		}
	}

	dcb.horizontalScaling = &v1.RadixHorizontalScaling{
		MinReplicas:                     minReplicas,
		MaxReplicas:                     maxReplicas,
		RadixHorizontalScalingResources: radixHorizontalScalingResources,
	}
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
		Secrets:                 dcb.secrets,
		SecretRefs:              dcb.secretRefs,
		IngressConfiguration:    dcb.ingressConfiguration,
		EnvironmentVariables:    dcb.environmentVariables,
		DNSAppAlias:             dcb.dnsappalias,
		DNSExternalAlias:        dcb.externalAppAlias,
		ExternalDNS:             dcb.externalDNS,
		Resources:               dcb.resources,
		HorizontalScaling:       dcb.horizontalScaling,
		VolumeMounts:            dcb.volumeMounts,
		AlwaysPullImageOnDeploy: dcb.alwaysPullImageOnDeploy,
		Node:                    dcb.node,
		Authentication:          dcb.authentication,
		Identity:                dcb.identity,
		ReadOnlyFileSystem:      dcb.readOnlyFileSystem,
	}
}

// NewDeployComponentBuilder Constructor for component builder
func NewDeployComponentBuilder() DeployComponentBuilder {
	return &deployComponentBuilder{}
}
