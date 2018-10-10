package kube

import (
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

func (k *Kube) ApplySecret(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	secretName := secret.ObjectMeta.Name
	log.Infof("Applies secret %s in namespace %s", secretName, namespace)

	savedSecret, err := k.kubeClient.CoreV1().Secrets(namespace).Create(secret)
	if errors.IsAlreadyExists(err) {
		log.Infof("Updating secret %s that already exists in namespace %s.", secretName, namespace)
		savedSecret, err = k.kubeClient.CoreV1().Secrets(namespace).Update(secret)
	}

	if err != nil {
		log.Errorf("Failed to apply secret %s in namespace %s. %v", secretName, namespace, err)
		return nil, err
	}
	log.Infof("Applied secret %s in namespace %s", secretName, namespace)
	return savedSecret, nil
}

func (k *Kube) ApplySecretsForPipelines(radixRegistration *radixv1.RadixRegistration) error {
	log.Infof("Apply secrets for pipelines")
	buildNamespace := GetCiCdNamespace(radixRegistration)

	err := k.applyDockerSecretToBuildNamespace(buildNamespace)
	if err != nil {
		return err
	}
	err = k.applyGitDeployKeyToBuildNamespace(buildNamespace, radixRegistration)
	if err != nil {
		return err
	}
	return nil
}

func (k *Kube) applyGitDeployKeyToBuildNamespace(namespace string, radixRegistration *radixv1.RadixRegistration) error {
	secret, err := k.createNewGitDeployKey(namespace, radixRegistration)
	if err != nil {
		return err
	}

	_, err = k.ApplySecret(namespace, secret)
	return err
}

func (k *Kube) applyDockerSecretToBuildNamespace(buildNamespace string) error {
	dockerSecretForBuild, err := k.createNewDockerBuildSecret(buildNamespace)
	if err != nil {
		return err
	}

	_, err = k.ApplySecret(buildNamespace, dockerSecretForBuild)
	return err
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

func (k *Kube) createNewDockerBuildSecret(namespace string) (*corev1.Secret, error) {
	dockerSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get radix-docker secret from default. %v", err)
		return nil, err
	}
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
	return &secret, nil
}
