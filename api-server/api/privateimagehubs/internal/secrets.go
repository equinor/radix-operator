package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// UpdatePrivateImageHubsSecretsPassword update secret password
func UpdatePrivateImageHubsSecretsPassword(ctx context.Context, kubeUtil *kube.Kube, appName, server, password string) error {
	namespace := operatorutils.GetAppNamespace(appName)
	original, err := kubeUtil.GetSecret(ctx, namespace, defaults.PrivateImageHubSecretName)
	if err != nil {
		return fmt.Errorf("failed to get private image hub secret for app %s: %w", appName, err)
	}

	modified := original.DeepCopy()

	authConfig, err := applicationconfig.UnmarshalPrivateImageHubAuthConfig(modified.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return err
	}

	config, ok := authConfig.Auths[server]
	if !ok {
		return fmt.Errorf("private image hub secret does not contain config for server %s", server)
	}

	config.Password = password
	authConfig.Auths[server] = config
	authConfigData, err := applicationconfig.MarshalPrivateImageHubAuthConfig(authConfig)
	if err != nil {
		return err
	}

	modified.Data[corev1.DockerConfigJsonKey] = authConfigData

	if err = kubequery.PatchSecretMetadata(modified, server, time.Now()); err != nil {
		return err
	}

	_, err = kubeUtil.UpdateSecret(ctx, original, modified)
	return err

}

// GetPendingPrivateImageHubSecrets returns a list of private image hubs where secret value is not set
func GetPendingPrivateImageHubSecrets(secret *corev1.Secret) ([]string, error) {
	var pendingSecrets []string
	authConfig, err := applicationconfig.UnmarshalPrivateImageHubAuthConfig(secret.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return nil, err
	}

	for key, imageHub := range authConfig.Auths {
		if imageHub.Password == "" {
			pendingSecrets = append(pendingSecrets, key)
		}
	}
	return pendingSecrets, nil
}
