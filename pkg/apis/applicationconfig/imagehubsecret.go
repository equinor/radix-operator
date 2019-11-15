package applicationconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

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
	secret, _ := app.kubeclient.CoreV1().Secrets(ns).Get(defaults.PrivateImageHubSecretName, metav1.GetOptions{})
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
	secret, err := app.kubeclient.CoreV1().Secrets(ns).Get(defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return pendingSecrets, err
	}

	imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
	for key, imageHub := range imageHubs {
		if imageHub.Password == "" {
			pendingSecrets = append(pendingSecrets, key)
		}
	}
	return pendingSecrets, nil
}

func (app *ApplicationConfig) syncPrivateImageHubSecrets() error {
	ns := utils.GetAppNamespace(app.config.Name)
	secret, err := app.kubeclient.CoreV1().Secrets(ns).Get(defaults.PrivateImageHubSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	err = app.grantAccessToPrivateImageHubSecret()
	if err != nil {
		return err
	}

	secretValue := []byte{}
	if errors.IsNotFound(err) || secret == nil {
		secretValue, err = createImageHubsSecretValue(app.config.Spec.PrivateImageHubs)
	} else {
		// update if changes
		hasChanged := false
		imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
		if err != nil {
			return err
		}

		// remove configs that doesn't exist
		for server := range imageHubs {
			if app.config.Spec.PrivateImageHubs[server] == nil {
				delete(imageHubs, server)
				hasChanged = true
			}
		}

		// update existing configs
		for server, config := range app.config.Spec.PrivateImageHubs {
			if currentConfig, ok := imageHubs[server]; ok {
				if config.Username != currentConfig.Username || config.Email != currentConfig.Email {
					currentConfig.Username = config.Username
					currentConfig.Email = config.Email
					imageHubs[server] = currentConfig
					hasChanged = true
				}
			} else {
				imageHubs[server] = secretImageHub{
					Username: config.Username,
					Email:    config.Email,
				}
				hasChanged = true
			}
		}
		if !hasChanged {
			return nil
		}
		secretValue, err = getImageHubsSecretValue(imageHubs)
	}

	if err != nil {
		return err
	}
	return applyPrivateImageHubSecret(app.kubeutil, ns, app.config.Name, secretValue)
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

// represent the secret of type docker-config
type secretImageHubsJSON struct {
	Auths secretImageHubs `json:"auths"`
}

type secretImageHubs map[string]secretImageHub

type secretImageHub struct {
	// +optional
	Username string `json:"username"`
	// +optional
	Password string `json:"password"`
	// +optional
	Email string `json:"email"`
	// +optional
	Auth string `json:"auth"`
}

// getImageHubSecretValue gets imagehub secret value
func getImageHubSecretValue(value []byte) (secretImageHubs, error) {
	secretValue := secretImageHubsJSON{}
	err := json.Unmarshal(value, &secretValue)
	if err != nil {
		return nil, err
	}

	return secretValue.Auths, nil
}

// createImageHubsSecretValue turn PrivateImageHubEntries into a json string correctly formated for k8s and ImagePullSecrets
func createImageHubsSecretValue(imagehubs v1.PrivateImageHubEntries) ([]byte, error) {
	imageHubs := secretImageHubs(map[string]secretImageHub{})

	for server, config := range imagehubs {
		pwd := ""
		imageHub := secretImageHub{
			Username: config.Username,
			Email:    config.Email,
			Password: pwd,
		}
		imageHubs[server] = imageHub
	}
	return getImageHubsSecretValue(imageHubs)
}

// getImageHubsSecretValue turn SecretImageHubs into a correctly formated secret for k8s ImagePullSecrets
func getImageHubsSecretValue(imageHubs secretImageHubs) ([]byte, error) {
	for server, config := range imageHubs {
		config.Auth = encodeAuthField(config.Username, config.Password)
		imageHubs[server] = config
	}

	imageHubJSON := secretImageHubsJSON{
		Auths: imageHubs,
	}
	return json.Marshal(imageHubJSON)
}

func encodeAuthField(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
