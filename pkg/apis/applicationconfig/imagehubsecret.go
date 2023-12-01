package applicationconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-common/pkg/docker"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getKubeDAnnotation(appName string) string {
	key, value := GetKubeDPrivateImageHubAnnotationValues(appName)
	return fmt.Sprintf("%s=%s", key, value)
}

// GetKubeDPrivateImageHubAnnotationValues gets value and key to use for namespace annotation to pick up private image hubs
func GetKubeDPrivateImageHubAnnotationValues(appName string) (key, value string) {
	return fmt.Sprintf("%s-sync", defaults.PrivateImageHubSecretName), appName
}

// UpdatePrivateImageHubsSecretsPassword updates the private image hub secret
func (app *ApplicationConfig) UpdatePrivateImageHubsSecretsPassword(server, password string) error {
	ns := utils.GetAppNamespace(app.config.Name)
	secret, _ := app.kubeutil.GetSecret(ns, defaults.PrivateImageHubSecretName)
	if secret == nil {
		return fmt.Errorf("private image hub secret does not exist for app %s", app.config.Name)
	}

	imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return err
	}

	if config, ok := imageHubs[server]; ok {
		config.Password = password
		imageHubs[server] = config
		secretValue, err := getImageHubsSecretValue(imageHubs)
		if err != nil {
			return err
		}
		return applyPrivateImageHubSecret(app.kubeutil, ns, app.config.Name, secretValue)
	}
	return fmt.Errorf("private image hub secret does not contain config for server %s", server)
}

// GetPendingPrivateImageHubSecrets returns a list of private image hubs where secret value is not set
func (app *ApplicationConfig) GetPendingPrivateImageHubSecrets() ([]string, error) {
	appName := app.config.Name
	pendingSecrets := []string{}
	ns := utils.GetAppNamespace(appName)
	secret, err := app.kubeutil.GetSecret(ns, defaults.PrivateImageHubSecretName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return nil, err
	}

	for key, imageHub := range imageHubs {
		if imageHub.Password == "" {
			pendingSecrets = append(pendingSecrets, key)
		}
	}
	return pendingSecrets, nil
}

func (app *ApplicationConfig) syncPrivateImageHubSecrets() error {
	namespace := utils.GetAppNamespace(app.config.Name)
	secret, err := app.kubeutil.GetSecret(namespace, defaults.PrivateImageHubSecretName)
	if err != nil && !errors.IsNotFound(err) {
		log.Warnf("failed to get private image hub secret %v", err)
		return err
	}

	var secretValue []byte
	if errors.IsNotFound(err) || secret == nil {
		secretValue, err = createImageHubsSecretValue(app.config.Spec.PrivateImageHubs)
		if err != nil {
			log.Warnf("failed to create private image hub secret %v", err)
			return err
		}
	} else {
		// update if changes
		imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
		if err != nil {
			log.Warnf("failed to get private image hub secret value %v", err)
			return err
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

		secretValue, err = getImageHubsSecretValue(imageHubs)
		if err != nil {
			log.Warnf("failed to update private image hub secret %v", err)
			return err
		}
	}
	err = applyPrivateImageHubSecret(app.kubeutil, namespace, app.config.Name, secretValue)
	if err != nil {
		return nil
	}

	err = utils.GrantAppReaderAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubReaderRoleName, defaults.PrivateImageHubSecretName)
	if err != nil {
		log.Warnf("failed to grant reader access to private image hub secret %v", err)
		return err
	}

	err = utils.GrantAppAdminAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubSecretName, defaults.PrivateImageHubSecretName)
	if err != nil {
		log.Warnf("failed to grant access to private image hub secret %v", err)
		return err
	}

	return app.grantPipelineAccessToSecret(namespace, defaults.PrivateImageHubSecretName)
}

// applyPrivateImageHubSecret create a private image hub secret based on SecretTypeDockerConfigJson
func applyPrivateImageHubSecret(kubeutil *kube.Kube, ns, appName string, secretValue []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeDockerConfigJson,
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.PrivateImageHubSecretName,
			Namespace: ns,
			Annotations: map[string]string{
				"kubed.appscode.com/sync": getKubeDAnnotation(appName),
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

	_, err := kubeutil.ApplySecret(ns, &secret)
	if err != nil {
		log.Warnf("Failed to create private image hub secrets for ns %s", ns)
		return err
	}
	return nil
}

// getImageHubSecretValue gets imagehub secret value
func getImageHubSecretValue(value []byte) (docker.Auths, error) {
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
	return getImageHubsSecretValue(imageHubs)
}

// getImageHubsSecretValue turn SecretImageHubs into a correctly formated secret for k8s ImagePullSecrets
func getImageHubsSecretValue(imageHubs docker.Auths) ([]byte, error) {
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
