package kube

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
)

type EnvironmentVariablesSourceDecorator interface {
	GetClusterName() (string, error)
	GetContainerRegistry() (string, error)
	GetDnsZone() (string, error)
	GetClusterType() (string, error)
}

type RadixApplicationEnvironmentVariablesSourceDecorator struct{}
type RadixOperatorEnvironmentVariablesSourceDecorator struct {
	kubeutil *Kube
}

func (envVarsSource *RadixApplicationEnvironmentVariablesSourceDecorator) GetClusterName() (string, error) {
	return getEnvVar(defaults.ClusternameEnvironmentVariable)
}

func (envVarsSource *RadixApplicationEnvironmentVariablesSourceDecorator) GetContainerRegistry() (string, error) {
	return getEnvVar(defaults.ContainerRegistryEnvironmentVariable)
}

func (envVarsSource *RadixApplicationEnvironmentVariablesSourceDecorator) GetDnsZone() (string, error) {
	return getEnvVar(defaults.RadixDNSZoneEnvironmentVariable)
}

func (envVarsSource *RadixApplicationEnvironmentVariablesSourceDecorator) GetClusterType() (string, error) {
	return getEnvVar(defaults.RadixClusterTypeEnvironmentVariable)
}

func (envVarsSource *RadixOperatorEnvironmentVariablesSourceDecorator) GetClusterName() (string, error) {
	clusterName, err := envVarsSource.kubeutil.GetClusterName()
	if err != nil {
		return "", fmt.Errorf("failed to get cluster name from ConfigMap: %v", err)
	}
	return clusterName, nil
}

func (envVarsSource *RadixOperatorEnvironmentVariablesSourceDecorator) GetContainerRegistry() (string, error) {
	containerRegistry, err := envVarsSource.kubeutil.GetContainerRegistry()
	if err != nil {
		return "", fmt.Errorf("failed to get container registry from ConfigMap: %v", err)
	}
	return containerRegistry, nil
}

func (envVarsSource *RadixOperatorEnvironmentVariablesSourceDecorator) GetDnsZone() (string, error) {
	return getEnvVar(defaults.OperatorDNSZoneEnvironmentVariable)
}

func (envVarsSource *RadixOperatorEnvironmentVariablesSourceDecorator) GetClusterType() (string, error) {
	return getEnvVar(defaults.OperatorClusterTypeEnvironmentVariable)
}
