package gateway

import (
	"errors"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	ErrComponentNotPublic = errors.New("component is not public")
)

// BuildBackendRefForComponent Builds backend reference for a component
func BuildBackendRefForComponent(component radixv1.RadixCommonDeployComponent) (gatewayapiv1.HTTPBackendRef, error) {

	servicePort, ok := component.GetPublicPortNumber()
	if !ok {
		return gatewayapiv1.HTTPBackendRef{}, ErrComponentNotPublic
	}

	return createBackendRef(component.GetName(), servicePort), nil
}

// BuildBackendRefForComponent Builds backend reference for a component
func BuildBackendRefForComponentOauth2Service(component radixv1.RadixCommonDeployComponent) gatewayapiv1.HTTPBackendRef {
	serviceName := utils.GetAuxOAuthProxyComponentServiceName(component.GetName())
	return createBackendRef(serviceName, defaults.OAuthProxyPortNumber)
}

func createBackendRef(serviceName string, servicePort int32) gatewayapiv1.HTTPBackendRef {
	return gatewayapiv1.HTTPBackendRef{
		BackendRef: gatewayapiv1.BackendRef{
			Weight: new(int32(1)),
			BackendObjectReference: gatewayapiv1.BackendObjectReference{
				Group: new(gatewayapiv1.Group("")),
				Kind:  new(gatewayapiv1.Kind("Service")),
				Name:  gatewayapiv1.ObjectName(serviceName),
				Port:  &servicePort,
			},
		},
	}
}
