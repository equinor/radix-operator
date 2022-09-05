package utils

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// DeployCommonComponentBuilder Handles construction of v1.RadixCommonDeployComponent builder
type DeployCommonComponentBuilder interface {
	WithName(string) DeployCommonComponentBuilder
	WithVolumeMounts(...v1.RadixVolumeMount) DeployCommonComponentBuilder
	BuildComponent() v1.RadixCommonDeployComponent
}

type deployCommonComponentBuilder struct {
	name         string
	volumeMounts []v1.RadixVolumeMount
	factory      v1.RadixCommonDeployComponentFactory
}

func (dcb *deployCommonComponentBuilder) WithName(name string) DeployCommonComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployCommonComponentBuilder) WithVolumeMounts(volumeMounts ...v1.RadixVolumeMount) DeployCommonComponentBuilder {
	dcb.volumeMounts = volumeMounts
	return dcb
}

func (dcb *deployCommonComponentBuilder) BuildComponent() v1.RadixCommonDeployComponent {
	component := dcb.factory.Create()
	component.SetName(dcb.name)
	component.SetVolumeMounts(dcb.volumeMounts)
	return component
}

// NewDeployCommonComponentBuilder Constructor for component builder
func NewDeployCommonComponentBuilder(factory v1.RadixCommonDeployComponentFactory) DeployCommonComponentBuilder {
	return &deployCommonComponentBuilder{
		factory: factory,
	}
}
