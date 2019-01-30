package application

import (
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DockerConfigJson struct {
	Authentication map[string]interface{} `json:"auths"`
}

const (
	spACRSecretName = "radix-sp-acr-azure" //also defined in ../pipeline-runner/build/build.go
)

// ApplySecretsForPipelines creates secrets needed by pipeline to run
func (app Application) applySecretsForPipelines() error {
	radixRegistration := app.registration

	log.Infof("Apply secrets for pipelines")
	buildNamespace := utils.GetAppNamespace(radixRegistration.Name)

	err := app.applyDockerSecretToBuildNamespace(buildNamespace)
	if err != nil {
		return err
	}
	err = app.applyGitDeployKeyToBuildNamespace(buildNamespace)
	if err != nil {
		return err
	}
	err = app.applyServicePrincipalACRSecretToBuildNamespace(buildNamespace)
	if err != nil {
		log.Warnf("Failed to apply service principle acr secret (%s) to namespace %s", spACRSecretName, buildNamespace)
	}
	return nil
}

func (app Application) applyGitDeployKeyToBuildNamespace(namespace string) error {
	radixRegistration := app.registration
	secret, err := createNewGitDeployKey(app.kubeclient, namespace, radixRegistration.Spec.DeployKey)
	if err != nil {
		return err
	}

	_, err = app.kubeutil.ApplySecret(namespace, secret)
	return err
}

func (app Application) applyDockerSecretToBuildNamespace(buildNamespace string) error {
	dockerSecretForBuild, err := createNewDockerBuildSecret(app.kubeclient, buildNamespace)
	if err != nil {
		return err
	}

	_, err = app.kubeutil.ApplySecret(buildNamespace, dockerSecretForBuild)
	return err
}

func (app Application) applyServicePrincipalACRSecretToBuildNamespace(buildNamespace string) error {
	servicePrincipalSecretForBuild, err := createNewServicePrincipalACRSecret(app.kubeclient, buildNamespace)
	if err != nil {
		return err
	}

	_, err = app.kubeutil.ApplySecret(buildNamespace, servicePrincipalSecretForBuild)
	return err
}

func createNewDockerBuildSecret(kubeclient kubernetes.Interface, namespace string) (*corev1.Secret, error) {
	dockerSecret, err := kubeclient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
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

func createNewGitDeployKey(kubeclient kubernetes.Interface, namespace, deployKey string) (*corev1.Secret, error) {
	knownHostsSecret, err := kubeclient.CoreV1().Secrets("default").Get("radix-known-hosts-git", metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get known hosts secret. %v", err)
		return nil, err
	}

	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-ssh-keys",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"id_rsa":      []byte(deployKey),
			"known_hosts": knownHostsSecret.Data["known_hosts"],
		},
	}
	return &secret, nil
}

func createNewServicePrincipalACRSecret(kubeclient kubernetes.Interface, namespace string) (*corev1.Secret, error) {
	servicePrincipalSecret, err := kubeclient.CoreV1().Secrets("default").Get(spACRSecretName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get %s secret from default. %v", spACRSecretName, err)
		return nil, err
	}
	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      spACRSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"sp_credentials.json": servicePrincipalSecret.Data["sp_credentials.json"],
		},
	}
	return &secret, nil
}
