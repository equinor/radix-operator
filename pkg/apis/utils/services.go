package utils

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetAuxiliaryComponentServiceName returns service name for auxiliary component, e.g. the oauth proxy
func GetAuxiliaryComponentServiceName(componentName string, auxSuffix string) string {
	return fmt.Sprintf("%s-%s", componentName, auxSuffix)
}

// BuildServicePorts transforms a Radix ComponentPort list toa  ServicePort list for use with a Kubernetes Service
func BuildServicePorts(componentPorts []radixv1.ComponentPort) []corev1.ServicePort {

	var ports []corev1.ServicePort
	for _, port := range componentPorts {
		servicePort := corev1.ServicePort{
			Name:       port.Name,
			Port:       port.Port,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(int(port.Port)),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
