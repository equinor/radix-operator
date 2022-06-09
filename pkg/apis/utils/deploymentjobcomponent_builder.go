package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeployJobComponentBuilder Handles construction of RD job component
type DeployJobComponentBuilder interface {
	WithName(string) DeployJobComponentBuilder
	WithImage(string) DeployJobComponentBuilder
	WithPort(string, int32) DeployJobComponentBuilder
	WithPorts([]v1.ComponentPort) DeployJobComponentBuilder
	WithEnvironmentVariable(string, string) DeployJobComponentBuilder
	WithEnvironmentVariables(map[string]string) DeployJobComponentBuilder
	WithMonitoring(bool) DeployJobComponentBuilder
	WithMonitoringConfig(v1.MonitoringConfig) DeployJobComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) DeployJobComponentBuilder
	WithResourceRequestsOnly(map[string]string) DeployJobComponentBuilder
	WithResource(map[string]string, map[string]string) DeployJobComponentBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) DeployJobComponentBuilder
	WithNodeGpu(gpu string) DeployJobComponentBuilder
	WithNodeGpuCount(gpuCount string) DeployJobComponentBuilder
	WithSecrets([]string) DeployJobComponentBuilder
	WithSecretRefs(v1.RadixSecretRefs) DeployJobComponentBuilder
	WithSchedulerPort(*int32) DeployJobComponentBuilder
	WithPayloadPath(*string) DeployJobComponentBuilder
	WithRunAsNonRoot(bool) DeployJobComponentBuilder
	WithTimeLimitSeconds(*int64) DeployJobComponentBuilder
	BuildJobComponent() v1.RadixDeployJobComponent
}

type deployJobComponentBuilder struct {
	name                    string
	image                   string
	ports                   []v1.ComponentPort
	environmentVariables    map[string]string
	monitoring              bool
	monitoringConfig        v1.MonitoringConfig
	alwaysPullImageOnDeploy bool
	secrets                 []string
	secretRefs              v1.RadixSecretRefs
	resources               v1.ResourceRequirements
	volumeMounts            []v1.RadixVolumeMount
	schedulerPort           *int32
	payloadPath             *string
	node                    v1.RadixNode
	runAsNonRoot            bool
	timeLimitSeconds        *int64
}

func (dcb *deployJobComponentBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) DeployJobComponentBuilder {
	dcb.volumeMounts = volumeMounts
	return dcb
}

func (dcb *deployJobComponentBuilder) WithNodeGpu(gpu string) DeployJobComponentBuilder {
	dcb.node.Gpu = gpu
	return dcb
}

func (dcb *deployJobComponentBuilder) WithNodeGpuCount(gpuCount string) DeployJobComponentBuilder {
	dcb.node.GpuCount = gpuCount
	return dcb
}

func (dcb *deployJobComponentBuilder) WithResourceRequestsOnly(request map[string]string) DeployJobComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Requests: request,
	}
	return dcb
}

func (dcb *deployJobComponentBuilder) WithResource(request map[string]string, limit map[string]string) DeployJobComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return dcb
}

func (dcb *deployJobComponentBuilder) WithName(name string) DeployJobComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployJobComponentBuilder) WithImage(image string) DeployJobComponentBuilder {
	dcb.image = image
	return dcb
}

func (dcb *deployJobComponentBuilder) WithPort(name string, port int32) DeployJobComponentBuilder {
	if dcb.ports == nil {
		dcb.ports = make([]v1.ComponentPort, 0)
	}

	dcb.ports = append(dcb.ports, v1.ComponentPort{Name: name, Port: port})
	return dcb
}

func (dcb *deployJobComponentBuilder) WithPorts(ports []v1.ComponentPort) DeployJobComponentBuilder {
	for i := range ports {
		dcb.WithPort(ports[i].Name, ports[i].Port)
	}
	return dcb
}

func (dcb *deployJobComponentBuilder) WithMonitoring(monitoring bool) DeployJobComponentBuilder {
	dcb.monitoring = monitoring
	return dcb
}

func (dcb *deployJobComponentBuilder) WithMonitoringConfig(monitoringConfig v1.MonitoringConfig) DeployJobComponentBuilder {
	dcb.monitoringConfig = monitoringConfig
	return dcb
}

func (dcb *deployJobComponentBuilder) WithAlwaysPullImageOnDeploy(val bool) DeployJobComponentBuilder {
	dcb.alwaysPullImageOnDeploy = val
	return dcb
}

func (dcb *deployJobComponentBuilder) WithEnvironmentVariable(name string, value string) DeployJobComponentBuilder {
	if dcb.environmentVariables == nil {
		dcb.environmentVariables = make(map[string]string)
	}

	dcb.environmentVariables[name] = value
	return dcb
}

func (dcb *deployJobComponentBuilder) WithEnvironmentVariables(environmentVariables map[string]string) DeployJobComponentBuilder {
	dcb.environmentVariables = environmentVariables
	return dcb
}

func (dcb *deployJobComponentBuilder) WithSecrets(secrets []string) DeployJobComponentBuilder {
	dcb.secrets = secrets
	return dcb
}

func (dcb *deployJobComponentBuilder) WithSecretRefs(secretRefs v1.RadixSecretRefs) DeployJobComponentBuilder {
	dcb.secretRefs = secretRefs
	return dcb
}

func (dcb *deployJobComponentBuilder) WithSchedulerPort(port *int32) DeployJobComponentBuilder {
	dcb.schedulerPort = port
	return dcb
}

func (dcb *deployJobComponentBuilder) WithPayloadPath(path *string) DeployJobComponentBuilder {
	dcb.payloadPath = path
	return dcb
}

func (dcb *deployJobComponentBuilder) WithRunAsNonRoot(runAsNonRoot bool) DeployJobComponentBuilder {
	dcb.runAsNonRoot = runAsNonRoot
	return dcb
}

func (dcb *deployJobComponentBuilder) WithTimeLimitSeconds(timeLimitSeconds *int64) DeployJobComponentBuilder {
	dcb.timeLimitSeconds = timeLimitSeconds
	return dcb
}

func (dcb *deployJobComponentBuilder) BuildJobComponent() v1.RadixDeployJobComponent {
	var payload *v1.RadixJobComponentPayload
	if dcb.payloadPath != nil {
		payload = &v1.RadixJobComponentPayload{Path: *dcb.payloadPath}
	}

	return v1.RadixDeployJobComponent{
		Image:                   dcb.image,
		Name:                    dcb.name,
		Ports:                   dcb.ports,
		Monitoring:              dcb.monitoring,
		MonitoringConfig:        dcb.monitoringConfig,
		Secrets:                 dcb.secrets,
		SecretRefs:              dcb.secretRefs,
		EnvironmentVariables:    dcb.environmentVariables,
		Resources:               dcb.resources,
		VolumeMounts:            dcb.volumeMounts,
		SchedulerPort:           dcb.schedulerPort,
		Payload:                 payload,
		AlwaysPullImageOnDeploy: dcb.alwaysPullImageOnDeploy,
		Node:                    dcb.node,
		RunAsNonRoot:            dcb.runAsNonRoot,
		TimeLimitSeconds:        dcb.timeLimitSeconds,
	}
}

// NewDeployJobComponentBuilder Constructor for jop component builder
func NewDeployJobComponentBuilder() DeployJobComponentBuilder {
	return &deployJobComponentBuilder{}
}
