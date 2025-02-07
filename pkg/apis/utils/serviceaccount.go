package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
)

// GetComponentServiceAccountName Gets unique name for component or job service account
func GetComponentServiceAccountName(componentName string) string {
	return fmt.Sprintf("%s-sa", componentName)
}

// GetSubPipelineServiceAccountName Gets unique name for component or job service account
func GetSubPipelineServiceAccountName(environmentName string) string {
	return fmt.Sprintf("subpipeline-%s-sa", environmentName)
}

// GetAuxOAuthServiceAccountName Gets unique name for aux oauth component or job service account
func GetAuxOAuthServiceAccountName(componentName string) string {
	return fmt.Sprintf("%s-%s-sa", componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
}
