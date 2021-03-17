package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeployJobComponentBuilder Handles construction of RD job component
type DeployJobComponentBuilder interface {
	WithName(string) DeployJobComponentBuilder
	WithImage(string) DeployJobComponentBuilder
	WithPort(string, int32) DeployJobComponentBuilder
	WithEnvironmentVariable(string, string) DeployJobComponentBuilder
	WithEnvironmentVariables(map[string]string) DeployJobComponentBuilder
	WithMonitoring(bool) DeployJobComponentBuilder
	WithAlwaysPullImageOnDeploy(bool) DeployJobComponentBuilder
	WithResourceRequestsOnly(map[string]string) DeployJobComponentBuilder
	WithResource(map[string]string, map[string]string) DeployJobComponentBuilder
	WithVolumeMounts([]v1.RadixVolumeMount) DeployJobComponentBuilder
	WithNodeGpu(gpu string) DeployJobComponentBuilder
	WithSecrets([]string) DeployJobComponentBuilder
	WithSchedulerPort(*int32) DeployJobComponentBuilder
	WithPayloadPath(*string) DeployJobComponentBuilder
	BuildJobComponent() v1.RadixDeployJobComponent
}

type deployJobComponentBuilder struct {
	name                    string
	image                   string
	ports                   map[string]int32
	environmentVariables    map[string]string
	monitoring              bool
	alwaysPullImageOnDeploy bool
	secrets                 []string
	resources               v1.ResourceRequirements
	volumeMounts            []v1.RadixVolumeMount
	node                    v1.RadixNode
	schedulerPort           *int32
	payloadPath             *string
}

func (dcb *deployJobComponentBuilder) WithVolumeMounts(volumeMounts []v1.RadixVolumeMount) DeployJobComponentBuilder {
	dcb.volumeMounts = volumeMounts
	return dcb
}

func (dcb *deployJobComponentBuilder) WithNodeGpu(gpu string) DeployJobComponentBuilder {
	dcb.node.Gpu = gpu
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
	dcb.ports[name] = port
	return dcb
}

func (dcb *deployJobComponentBuilder) WithMonitoring(monitoring bool) DeployJobComponentBuilder {
	dcb.monitoring = monitoring
	return dcb
}

func (dcb *deployJobComponentBuilder) WithAlwaysPullImageOnDeploy(val bool) DeployJobComponentBuilder {
	dcb.alwaysPullImageOnDeploy = val
	return dcb
}

func (dcb *deployJobComponentBuilder) WithEnvironmentVariable(name string, value string) DeployJobComponentBuilder {
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

func (dcb *deployJobComponentBuilder) WithSchedulerPort(port *int32) DeployJobComponentBuilder {
	dcb.schedulerPort = port
	return dcb
}

func (dcb *deployJobComponentBuilder) WithPayloadPath(path *string) DeployJobComponentBuilder {
	dcb.payloadPath = path
	return dcb
}

func (dcb *deployJobComponentBuilder) BuildJobComponent() v1.RadixDeployJobComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range dcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	var payload *v1.RadixJobComponentPayload
	if dcb.payloadPath != nil {
		payload = &v1.RadixJobComponentPayload{Path: *dcb.payloadPath}
	}

	return v1.RadixDeployJobComponent{
		Image:                   dcb.image,
		Name:                    dcb.name,
		Ports:                   componentPorts,
		Monitoring:              dcb.monitoring,
		Secrets:                 dcb.secrets,
		EnvironmentVariables:    dcb.environmentVariables,
		Resources:               dcb.resources,
		VolumeMounts:            dcb.volumeMounts,
		SchedulerPort:           dcb.schedulerPort,
		Payload:                 payload,
		AlwaysPullImageOnDeploy: dcb.alwaysPullImageOnDeploy,
		Node:                    dcb.node,
	}
}

// NewDeployJobComponentBuilder Constructor for jop component builder
func NewDeployJobComponentBuilder() DeployJobComponentBuilder {
	return &deployJobComponentBuilder{
		ports:                make(map[string]int32),
		environmentVariables: make(map[string]string),
	}
}
