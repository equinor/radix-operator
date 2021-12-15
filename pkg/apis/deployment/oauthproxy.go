package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func (deploy *Deployment) createOrUpdateOAuthProxy(component v1.RadixCommonDeployComponent) error {
	return deploy.oauthProxyResourceManager.Sync(component)
}
