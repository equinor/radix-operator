package applicationconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrivateImageHubSecretName contain secret with all private image hubs credentials for an app
const PrivateImageHubSecretName = "radix-private-image-hubs"

func getKubeDAnnotation(namespace string) string {
	key, value := GetKubeDPrivateImageHubAnnotationValues(namespace)
	return fmt.Sprintf("%s=%s", key, value)
}

// GetKubeDPrivateImageHubAnnotationValues gets value and key to use for namespace annotation to pick up private image hubs
func GetKubeDPrivateImageHubAnnotationValues(namespace string) (key, value string) {
	return fmt.Sprintf("%s-sync", PrivateImageHubSecretName), namespace
}

// UpdatePrivateImageHubsSecretsPassword updates the private image hub secret
func (app *ApplicationConfig) UpdatePrivateImageHubsSecretsPassword(server, password string) error {
	ns := utils.GetAppNamespace(app.config.Name)
	secret, _ := app.kubeclient.CoreV1().Secrets(ns).Get(PrivateImageHubSecretName, metav1.GetOptions{})
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
		return applyPrivateImageHubSecret(app.kubeutil, ns, secretValue)
	}
	return fmt.Errorf("private image hub secret does not contain config for server %s", server)
}

func (app *ApplicationConfig) syncPrivateImageHubSecrets() error {
	ns := utils.GetAppNamespace(app.config.Name)
	if app.config.Spec.PrivateImageHubs == nil {
		return nil
	}

	secret, _ := app.kubeclient.CoreV1().Secrets(ns).Get(PrivateImageHubSecretName, metav1.GetOptions{})
	secretValue, err := []byte{}, error(nil)
	if secret != nil {
		// update if changes
		hasChanged := false
		imageHubs, err := getImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
		if err != nil {
			return err
		}
		// update existing configs
		for server, config := range app.config.Spec.PrivateImageHubs {
			if currentConfig, ok := imageHubs[server]; ok {
				if config.Username != currentConfig.Username || config.Email != currentConfig.Email {
					currentConfig.Username = config.Username
					currentConfig.Email = config.Email
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
		// remove configs that doesn't exist
		for server := range imageHubs {
			if app.config.Spec.PrivateImageHubs[server] == nil {
				delete(imageHubs, server) // TODO can item be deleted while iterated over?
				hasChanged = true
			}
		}
		if !hasChanged {
			return nil
		}
		secretValue, err = getImageHubsSecretValue(imageHubs)
	} else {
		secretValue, err = createImageHubsSecretValue(app.config.Spec.PrivateImageHubs)
	}

	if err != nil {
		return err
	}
	return applyPrivateImageHubSecret(app.kubeutil, ns, secretValue)
}

// applyPrivateImageHubSecret create a private image hub secret based on SecretTypeDockerConfigJson
func applyPrivateImageHubSecret(kubeutil *kube.Kube, ns string, secretValue []byte) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeDockerConfigJson,
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrivateImageHubSecretName,
			Namespace: ns,
			Annotations: map[string]string{
				"kubed.appscode.com/sync": getKubeDAnnotation(ns),
			},
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: secretValue,
		},
	}
	_, err := kubeutil.ApplySecret(ns, &secret)
	if err != nil {
		log.Warnf("Failed to create private image hub secrets for ns %s", ns)
		return err
	}
	return nil
}

// represent the secret of type docker-config
type secretImageHubsJson struct {
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
	secretValue := secretImageHubsJson{}
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

	imageHubJSON := secretImageHubsJson{
		Auths: imageHubs,
	}
	return json.Marshal(imageHubJSON)
}

func encodeAuthField(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
