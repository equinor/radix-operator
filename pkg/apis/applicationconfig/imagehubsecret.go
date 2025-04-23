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

func (app *ApplicationConfig) syncPrivateImageHubSecrets(ctx context.Context) error {
	currentSecret, desiredSecret, err := app.getCurrentAndDesiredImageHubSecret(ctx)
	if err != nil {
		return fmt.Errorf("failed get current and desired private image hub secret: %w", err)
	}

	if currentSecret != nil {
		if _, err := app.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
			return fmt.Errorf("failed to update private image hub secret: %w", err)
		}
	} else {
		if _, err := app.kubeutil.CreateSecret(ctx, desiredSecret.Namespace, desiredSecret); err != nil {
			return fmt.Errorf("failed to create private image hub secret: %w", err)
		}
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

func (app *ApplicationConfig) getCurrentAndDesiredImageHubSecret(ctx context.Context) (current, desired *corev1.Secret, err error) {
	ns := utils.GetAppNamespace(app.config.Name)
	currentInternal, err := app.kubeutil.GetSecret(ctx, ns, defaults.PrivateImageHubSecretName)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to get private image hub secret: %w", err)
		}
		desired = &corev1.Secret{
			Type: corev1.SecretTypeDockerConfigJson,
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.PrivateImageHubSecretName,
				Namespace: ns,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	desired.Labels = map[string]string{
		kube.RadixAppLabel: app.config.Name,
	}
	if desired.Annotations == nil {
		desired.Annotations = map[string]string{}
	}

	// Set annotation for Kubernetes Replicator
	desired.Annotations["replicator.v1.mittwald.de/replicate-to-matching"] = fmt.Sprintf("%s-sync=%s", defaults.PrivateImageHubSecretName, app.config.Name)
	// Deleted deprecated Kubernetes Config Syncer annotation
	delete(desired.Annotations, "kubed.appscode.com/sync")

	if err := setPrivateImageHubSecretData(desired, app.config.Spec.PrivateImageHubs); err != nil {
		return nil, nil, fmt.Errorf("failed to set private image hub data: %w", err)
	}

	return current, desired, nil
}

func setPrivateImageHubSecretData(secret *corev1.Secret, privateImageHubs v1.PrivateImageHubEntries) error {
	authConfig := docker.AuthConfig{}
	if existingData, ok := secret.Data[corev1.DockerConfigJsonKey]; ok {
		tmpAuthConfig, err := UnmarshalPrivateImageHubAuthConfig(existingData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal existing private image hub data: %w", err)
		}
		authConfig = tmpAuthConfig
	}
	if authConfig.Auths == nil {
		authConfig.Auths = docker.Auths{}
	}

	// Remove servers from auths no longer in the private image hubs spec
	for server := range authConfig.Auths {
		if _, exist := privateImageHubs[server]; !exist {
			delete(authConfig.Auths, server)
		}
	}

	// Update existing/add missing private image hubs from spec
	for server, config := range privateImageHubs {
		cred := authConfig.Auths[server]
		cred.Username = config.Username
		cred.Email = config.Email
		authConfig.Auths[server] = cred
	}

	authData, err := MarshalPrivateImageHubAuthConfig(authConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal private image hub data: %w", err)
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[corev1.DockerConfigJsonKey] = authData

	return nil
}

// UnmarshalPrivateImageHubAuthConfig unmarshals DockerConfigJsonKey secret data
func UnmarshalPrivateImageHubAuthConfig(value []byte) (docker.AuthConfig, error) {
	authConfig := docker.AuthConfig{}
	err := json.Unmarshal(value, &authConfig)
	return authConfig, err
}

// MarshalPrivateImageHubAuthConfig marshals AuthConfig as valid secret data to be used as ImagePullSecrets
func MarshalPrivateImageHubAuthConfig(authConfig docker.AuthConfig) ([]byte, error) {
	for server, config := range authConfig.Auths {
		config.Auth = encodeAuthField(config.Username, config.Password)
		authConfig.Auths[server] = config
	}

	return json.Marshal(authConfig)
}

func encodeAuthField(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
