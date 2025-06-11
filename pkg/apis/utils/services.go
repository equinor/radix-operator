package utils

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetServicePorts transforms a Radix ComponentPort list toa  ServicePort list for use with a Kubernetes Service
func GetServicePorts(componentPorts []radixv1.ComponentPort) []corev1.ServicePort {

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

// GetAuxOAuthProxyComponentServiceName returns service name for auxiliary OAuth proxy component
func GetAuxOAuthProxyComponentServiceName(componentName string) string {
	return fmt.Sprintf("%s-%s", componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
}
