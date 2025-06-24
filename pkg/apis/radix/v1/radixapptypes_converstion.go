package v1

import (
	"fmt"

	v2 "github.com/equinor/radix-operator/pkg/apis/radix/v2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *RadixApplication) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v2.RadixApplication)
	if !ok {
		return fmt.Errorf("expected a *v2.RadixApplication object but got %T", dstRaw)
	}
	fmt.Println("running ConvertTo")

	dst.ObjectMeta = src.ObjectMeta

	dstComponents := make([]v2.RadixComponent, 0, len(src.Spec.Components))
	for _, srcComponent := range src.Spec.Components {
		dstComponents = append(dstComponents, convertComponentToV2(srcComponent))
	}
	dst.Spec.Components = dstComponents

	return nil
}

func convertComponentToV2(src RadixComponent) v2.RadixComponent {
	dst := v2.RadixComponent{
		Name:       src.Name,
		Replicas:   src.Replicas,
		Enabled:    src.Enabled,
		Identity:   convertIdentityToV2(src.Identity),
		Containers: convertComponentToV2Container(src),
	}

	return dst
}

func convertComponentToV2Container(src RadixComponent) []v2.RadixComponentContainer {
	container := v2.RadixComponentContainer{
		Name:           src.Name,
		SourceFolder:   src.SourceFolder,
		DockerfileName: src.DockerfileName,
		Image:          src.Image,
		ImageTagName:   src.ImageTagName,
		Monitoring:     src.Monitoring,
		Ports:          convertPortsToV2(src.Ports),
	}

	if len(src.PublicPort) > 0 {
		container.PublicPort = src.PublicPort
	} else if src.Public && len(src.Ports) > 0 {
		container.PublicPort = src.Ports[0].Name
	}

	return []v2.RadixComponentContainer{container}
}

func convertPortsToV2(src []ComponentPort) []v2.ComponentPort {
	dstPorts := make([]v2.ComponentPort, 0, len(src))

	for _, srcPort := range src {
		dstPorts = append(dstPorts, v2.ComponentPort{Name: srcPort.Name, Port: srcPort.Port})
	}

	return dstPorts
}

func convertIdentityToV2(src *Identity) *v2.Identity {
	if src == nil {
		return nil
	}

	return &v2.Identity{
		Azure: convertAzureIdentityToV2(src.Azure),
	}
}

func convertAzureIdentityToV2(src *AzureIdentity) *v2.AzureIdentity {
	if src == nil {
		return nil
	}

	return &v2.AzureIdentity{
		ClientId: src.ClientId,
	}
}

func (dst *RadixApplication) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v2.RadixApplication)
	if !ok {
		return fmt.Errorf("expected a *v2.RadixApplication object but got %T", srcRaw)
	}
	fmt.Println("running ConvertFrom")

	dst.ObjectMeta = src.ObjectMeta

	dstComponents := make([]RadixComponent, 0, len(src.Spec.Components))
	for _, srcComponent := range src.Spec.Components {
		dstComponents = append(dstComponents, convertComponentToV1(srcComponent))
	}
	dst.Spec.Components = dstComponents

	return nil
}

func convertComponentToV1(src v2.RadixComponent) RadixComponent {
	dst := RadixComponent{
		Name:     src.Name,
		Replicas: src.Replicas,
		Enabled:  src.Enabled,
		Identity: convertIdentityToV1(src.Identity),
	}

	if len(src.Containers) > 0 {
		srcContainer := src.Containers[0]
		dst.SourceFolder = srcContainer.SourceFolder
		dst.DockerfileName = srcContainer.DockerfileName
		dst.Image = srcContainer.Image
		dst.ImageTagName = srcContainer.ImageTagName
		dst.Monitoring = srcContainer.Monitoring
		dst.Ports = convertPortsToV1(srcContainer.Ports)
		dst.PublicPort = srcContainer.PublicPort
	}

	return dst
}

func convertIdentityToV1(src *v2.Identity) *Identity {
	if src == nil {
		return nil
	}

	return &Identity{
		Azure: convertAzureIdentityToV1(src.Azure),
	}
}

func convertAzureIdentityToV1(src *v2.AzureIdentity) *AzureIdentity {
	if src == nil {
		return nil
	}

	return &AzureIdentity{
		ClientId: src.ClientId,
	}
}

func convertPortsToV1(src []v2.ComponentPort) []ComponentPort {
	dstPorts := make([]ComponentPort, 0, len(src))

	for _, srcPort := range src {
		dstPorts = append(dstPorts, ComponentPort{Name: srcPort.Name, Port: srcPort.Port})
	}

	return dstPorts
}
