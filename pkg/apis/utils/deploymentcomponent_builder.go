package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeployComponentBuilder Handles construction of RD component
type DeployComponentBuilder interface {
	WithName(string) DeployComponentBuilder
	WithImage(string) DeployComponentBuilder
	WithPort(string, int32) DeployComponentBuilder
	WithEnvironmentVariable(string, string) DeployComponentBuilder
	WithEnvironmentVariables(map[string]string) DeployComponentBuilder
	// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublic(bool) DeployComponentBuilder
	WithPublicPort(string) DeployComponentBuilder
	WithMonitoring(bool) DeployComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) DeployComponentBuilder
	WithReplicas(*int) DeployComponentBuilder
	WithResourceRequestsOnly(map[string]string) DeployComponentBuilder
	WithResource(map[string]string, map[string]string) DeployComponentBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) DeployComponentBuilder
	WithIngressConfiguration(...string) DeployComponentBuilder
	WithSecrets([]string) DeployComponentBuilder
	WithDNSAppAlias(bool) DeployComponentBuilder
	WithDNSExternalAlias(string) DeployComponentBuilder
	WithHorizontalScaling(*int32, int32) DeployComponentBuilder
	WithRunAsNonRoot(bool) DeployComponentBuilder
	BuildComponent() v1.RadixDeployComponent
}

type deployComponentBuilder struct {
	name                 string
	runAsNonRoot         bool
	image                string
	ports                map[string]int32
	environmentVariables map[string]string
	// Deprecated: For backwards comptibility public is still supported, new code should use publicPort instead
	public                  bool
	publicPort              string
	monitoring              bool
	replicas                *int
	alwaysPullImageOnDeploy bool
	ingressConfiguration    []string
	secrets                 []string
	dnsappalias             bool
	externalAppAlias        []string
	resources               v1.ResourceRequirements
	horizontalScaling       *v1.RadixHorizontalScaling
	volumeMounts            []v1.RadixVolumeMount
}

func (dcb *deployComponentBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) DeployComponentBuilder {
	dcb.volumeMounts = volumeMounts
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

func (dcb *deployComponentBuilder) WithDNSExternalAlias(alias string) DeployComponentBuilder {
	if dcb.externalAppAlias == nil {
		dcb.externalAppAlias = make([]string, 0)
	}

	dcb.externalAppAlias = append(dcb.externalAppAlias, alias)
	return dcb
}

func (dcb *deployComponentBuilder) WithImage(image string) DeployComponentBuilder {
	dcb.image = image
	return dcb
}

func (dcb *deployComponentBuilder) WithPort(name string, port int32) DeployComponentBuilder {
	dcb.ports[name] = port
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

func (dcb *deployComponentBuilder) WithReplicas(replicas *int) DeployComponentBuilder {
	dcb.replicas = replicas
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariable(name string, value string) DeployComponentBuilder {
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

func (dcb *deployComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) DeployComponentBuilder {
	dcb.ingressConfiguration = ingressConfiguration
	return dcb
}

func (dcb *deployComponentBuilder) WithHorizontalScaling(minReplicas *int32, maxReplicas int32) DeployComponentBuilder {
	dcb.horizontalScaling = &v1.RadixHorizontalScaling{
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithRunAsNonRoot(runAsNonRoot bool) DeployComponentBuilder {
	dcb.runAsNonRoot = runAsNonRoot
	return dcb
}

func (dcb *deployComponentBuilder) BuildComponent() v1.RadixDeployComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range dcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	return v1.RadixDeployComponent{
		Image:                   dcb.image,
		RunAsNonRoot:            dcb.runAsNonRoot,
		Name:                    dcb.name,
		Ports:                   componentPorts,
		Public:                  dcb.public,
		PublicPort:              dcb.publicPort,
		Monitoring:              dcb.monitoring,
		Replicas:                dcb.replicas,
		Secrets:                 dcb.secrets,
		IngressConfiguration:    dcb.ingressConfiguration,
		EnvironmentVariables:    dcb.environmentVariables,
		DNSAppAlias:             dcb.dnsappalias,
		DNSExternalAlias:        dcb.externalAppAlias,
		Resources:               dcb.resources,
		HorizontalScaling:       dcb.horizontalScaling,
		VolumeMounts:            dcb.volumeMounts,
		AlwaysPullImageOnDeploy: dcb.alwaysPullImageOnDeploy,
	}
}

// NewDeployComponentBuilder Constructor for component builder
func NewDeployComponentBuilder() DeployComponentBuilder {
	return &deployComponentBuilder{
		ports:                make(map[string]int32),
		environmentVariables: make(map[string]string),
	}
}
