package defaults

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetAuxiliaryComponentServiceName returns service name for auxiliary component, e.g. the oauth proxy
func GetAuxiliaryComponentServiceName(componentName string, auxSuffix string) string {
	return fmt.Sprintf("%s-%s", componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
}

// GetAuxOAuthRedisServiceName returns service name for auxiliary OAuth redis component
func GetAuxOAuthRedisServiceName(componentName string) string {
	return fmt.Sprintf("%s-%s", componentName, radixv1.OAuthRedisAuxiliaryComponentSuffix)
}
