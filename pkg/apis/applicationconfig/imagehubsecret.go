package applicationconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-common/pkg/docker"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getSyncTargetAnnotation(appName string) string {
	return fmt.Sprintf("%s-sync=%s", defaults.PrivateImageHubSecretName, appName)
}

func (app *ApplicationConfig) syncPrivateImageHubSecrets(ctx context.Context) error {
	namespace := utils.GetAppNamespace(app.config.Name)
	secret, err := app.kubeutil.GetSecret(ctx, namespace, defaults.PrivateImageHubSecretName)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get private image hub secret: %W", err)
	}

	var secretValue []byte
	if kubeerrors.IsNotFound(err) || secret == nil {
		secretValue, err = createImageHubsSecretValue(app.config.Spec.PrivateImageHubs)
		if err != nil {
			return fmt.Errorf("failed to create private image hub secret: %w", err)
		}
	} else {
		// update if changes
		imageHubs, err := GetImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
		if err != nil {
			return fmt.Errorf("failed to get private image hub secret value: %w", err)
		}

		// remove configs that doesn't exist
		for server := range imageHubs {
			if app.config.Spec.PrivateImageHubs[server] == nil {
				delete(imageHubs, server)
			}
		}

		// update existing configs
		for server, config := range app.config.Spec.PrivateImageHubs {
			if currentConfig, ok := imageHubs[server]; ok {
				if config.Username != currentConfig.Username || config.Email != currentConfig.Email {
					currentConfig.Username = config.Username
					currentConfig.Email = config.Email
					imageHubs[server] = currentConfig
				}
			} else {
				imageHubs[server] = docker.Credential{
					Username: config.Username,
					Email:    config.Email,
				}
			}
		}

		secretValue, err = GetImageHubsSecretValue(imageHubs)
		if err != nil {
			return fmt.Errorf("failed to update private image hub secret: %w", err)
		}
	}
	err = ApplyPrivateImageHubSecret(ctx, app.kubeutil, namespace, app.config.Name, secretValue)
	if err != nil {
		return nil
	}

	err = utils.GrantAppReaderAccessToSecret(ctx, app.kubeutil, app.registration, defaults.PrivateImageHubReaderRoleName, defaults.PrivateImageHubSecretName)
	if err != nil {
		return fmt.Errorf("failed to grant reader access to private image hub secret: %w", err)
	}

	err = utils.GrantAppAdminAccessToSecret(ctx, app.kubeutil, app.registration, defaults.PrivateImageHubSecretName, defaults.PrivateImageHubSecretName)
	if err != nil {
		return fmt.Errorf("failed to grant access to private image hub secret: %w", err)
	}

	return nil
}

// ApplyPrivateImageHubSecret create a private image hub secret based on SecretTypeDockerConfigJson
func ApplyPrivateImageHubSecret(ctx context.Context, kubeutil *kube.Kube, ns, appName string, secretValue []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeDockerConfigJson,
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.PrivateImageHubSecretName,
			Namespace: ns,
			Annotations: map[string]string{
				"kubed.appscode.com/sync":                         getSyncTargetAnnotation(appName),
				"replicator.v1.mittwald.de/replicate-to-matching": getSyncTargetAnnotation(appName),
			},
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		Data: map[string][]byte{},
	}
	if secretValue != nil {
		secret.Data[corev1.DockerConfigJsonKey] = secretValue
	}

	_, err := kubeutil.ApplySecret(ctx, ns, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return fmt.Errorf("failed to create private image hub secrets in namespace %s: %w", ns, err)
	}
	return nil
}

// GetImageHubSecretValue gets imagehub secret value
func GetImageHubSecretValue(value []byte) (docker.Auths, error) {
	secretValue := docker.AuthConfig{}
	err := json.Unmarshal(value, &secretValue)
	if err != nil {
		return nil, err
	}

	return secretValue.Auths, nil
}

// createImageHubsSecretValue turn PrivateImageHubEntries into a json string correctly formated for k8s and ImagePullSecrets
func createImageHubsSecretValue(imagehubs v1.PrivateImageHubEntries) ([]byte, error) {
	imageHubs := docker.Auths{}

	for server, config := range imagehubs {
		pwd := ""
		imageHub := docker.Credential{
			Username: config.Username,
			Email:    config.Email,
			Password: pwd,
		}
		imageHubs[server] = imageHub
	}
	return GetImageHubsSecretValue(imageHubs)
}

// GetImageHubsSecretValue turn SecretImageHubs into a correctly formated secret for k8s ImagePullSecrets
func GetImageHubsSecretValue(imageHubs docker.Auths) ([]byte, error) {
	for server, config := range imageHubs {
		config.Auth = encodeAuthField(config.Username, config.Password)
		imageHubs[server] = config
	}

	imageHubJSON := docker.AuthConfig{
		Auths: imageHubs,
	}
	return json.Marshal(imageHubJSON)
}

func encodeAuthField(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
