package kube

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DockerConfigJson struct {
	Authentication map[string]interface{} `json:"auths"`
}

type ContainerRegistryCredentials struct {
	Server   string `json:""`
	User     string
	Password string
}

func (k *Kube) ApplySecretsForPipelines(radixRegistration *radixv1.RadixRegistration) error {
	log.Infof("Apply secrets for pipelines")
	buildNamespace := GetCiCdNamespace(radixRegistration)

	err := k.applyDockerSecretToBuildNamespace(buildNamespace, radixRegistration)
	if err != nil {
		return err
	}
	err = k.applyGitDeployKeyToBuildNamespace(buildNamespace, radixRegistration)
	if err != nil {
		return err
	}
	return nil
}

//RetrieveContainerRegistryCredentials retrieves the default container registry credentials from Kubernetes secret
func (k *Kube) RetrieveContainerRegistryCredentials() (*ContainerRegistryCredentials, error) {
	secret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve container credentials: %v", err)
	}
	credsJSON, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("Failed to read docker config from radix-docker")
	}

	var config DockerConfigJson
	err = json.Unmarshal(credsJSON, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal docker config: %v", err)
	}

	creds := &ContainerRegistryCredentials{}
	for key := range config.Authentication {
		creds.Server = key
		values := config.Authentication[key].(map[string]interface{})
		auth := values["auth"].(string)
		decoded, err := getDecodedCredentials(auth)
		if err != nil {
			return nil, err
		}

		credentials := strings.Split(decoded, ":")
		creds.User = credentials[0]
		creds.Password = credentials[1]
	}

	return creds, nil
}

func getDecodedCredentials(encodedCredentials string) (string, error) {
	creds, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return "", fmt.Errorf("Failed to decode credentials: %v", err)
	}
	return string(creds), nil
}

func (k *Kube) applyGitDeployKeyToBuildNamespace(namespace string, radixRegistration *radixv1.RadixRegistration) error {
	secret, err := k.createNewGitDeployKey(namespace, radixRegistration)
	if err != nil {
		return err
	}

	_, err = k.kubeClient.CoreV1().Secrets(namespace).Create(secret)
	if errors.IsAlreadyExists(err) {
		log.Infof("git deploykey file already exists")
		return nil
	}

	if err != nil {
		log.Errorf("Failed to create git-ssh-keys secret. %v", err)
		return err
	}
	return nil
}

func (k *Kube) createNewGitDeployKey(namespace string, radixRegistration *radixv1.RadixRegistration) (*corev1.Secret, error) {
	knownHostsSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-known-hosts-git", metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get known hosts secret. %v", err)
		return nil, err
	}

	idRsa := radixRegistration.Spec.DeployKey
	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-ssh-keys",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"id_rsa":      []byte(idRsa),
			"known_hosts": knownHostsSecret.Data["known_hosts"],
		},
	}
	return &secret, nil
}

func (k *Kube) applyDockerSecretToBuildNamespace(buildNamespace string, radixRegistration *radixv1.RadixRegistration) error {
	dockerSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get radix-docker secret from default. %v", err)
		return err
	}
	dockerSecretForBuild := createNewDockerBuildSecret(buildNamespace, dockerSecret)

	_, err = k.kubeClient.CoreV1().Secrets(buildNamespace).Create(&dockerSecretForBuild)
	if errors.IsAlreadyExists(err) {
		log.Infof("radix-docker file already exists")
		return nil
	}

	if err != nil {
		log.Errorf("Failed to create radix-docker in namespace %s", buildNamespace)
		return err
	}

	return nil
}

func createNewDockerBuildSecret(namespace string, dockerSecret *corev1.Secret) corev1.Secret {
	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"config.json": dockerSecret.Data[".dockerconfigjson"],
		},
	}
	return secret
}
