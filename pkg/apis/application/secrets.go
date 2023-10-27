package application

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplySecretsForPipelines creates secrets needed by pipeline to run
func (app Application) applySecretsForPipelines() error {
	radixRegistration := app.registration

	log.Debugf("Apply secrets for pipelines")
	buildNamespace := utils.GetAppNamespace(radixRegistration.Name)

	err := app.applyGitDeployKeyToBuildNamespace(buildNamespace)
	if err != nil {
		return err
	}
	err = app.applyServicePrincipalACRSecretToBuildNamespace(buildNamespace)
	if err != nil {
		log.Warnf("Failed to apply service principle acr secrets (%s, %s) to namespace %s", defaults.AzureACRServicePrincipleSecretName, defaults.AzureACRServicePrincipleBuildahSecretName, buildNamespace)
	}
	return nil
}

func (app Application) applyGitDeployKeyToBuildNamespace(namespace string) error {
	radixRegistration := app.registration
	secret, err := app.kubeutil.GetSecret(namespace, defaults.GitPrivateKeySecretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}

	deployKey := &utils.DeployKey{}

	privateKeyExists := app.gitPrivateKeyExists(secret) // checking if private key exists
	if privateKeyExists {
		deployKey, err = deriveKeyPairFromSecret(secret) // if private key exists, retrieve public key from secret
		if err != nil {
			return err
		}
	} else {
		// if private key secret does not exist, check if RR has private key
		deployKeyString := radixRegistration.Spec.DeployKey
		if deployKeyString == "" {
			// if RR does not have private key, generate new key pair
			deployKey, err = utils.GenerateDeployKey()
			if err != nil {
				return err
			}
		} else {
			// if RR has private key, retrieve both public and private key from RR
			deployKey.PublicKey = radixRegistration.Spec.DeployKeyPublic
			deployKey.PrivateKey = radixRegistration.Spec.DeployKey
		}
	}

	// create corresponding secret with private key
	secret, err = app.createNewGitDeployKey(namespace, deployKey.PrivateKey, radixRegistration)
	if err != nil {
		return err
	}
	_, err = app.kubeutil.ApplySecret(namespace, secret)
	if err != nil {
		return err
	}

	newCm := app.createGitPublicKeyConfigMap(namespace, deployKey.PublicKey, radixRegistration)
	_, err = app.kubeutil.CreateConfigMap(namespace, newCm)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			existingCm, err := app.kubeutil.GetConfigMap(namespace, defaults.GitPublicKeyConfigMapName)
			if err != nil {
				return err
			}
			err = app.kubeutil.ApplyConfigMap(namespace, existingCm, newCm)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func deriveKeyPairFromSecret(secret *corev1.Secret) (*utils.DeployKey, error) {
	privateKey := string(secret.Data[defaults.GitPrivateKeySecretKey])
	deployKey, err := utils.DeriveDeployKeyFromPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return deployKey, nil
}

func (app Application) applyServicePrincipalACRSecretToBuildNamespace(buildNamespace string) error {
	servicePrincipalSecretForBuild, err := app.createNewServicePrincipalACRSecret(buildNamespace, defaults.AzureACRServicePrincipleSecretName)
	if err != nil {
		return err
	}

	servicePrincipalSecretForBuildahBuild, err := app.createNewServicePrincipalACRSecret(buildNamespace, defaults.AzureACRServicePrincipleBuildahSecretName)
	if err != nil {
		return err
	}

	tokenSecretForBuildahCacheRegistry, err := app.createNewServicePrincipalACRSecret(buildNamespace, defaults.AzureACRTokenPasswordBuildahCacheSecretName)
	if err != nil {
		return err
	}

	// TODO: Ask @Nils: When does Radix Operator sync the secrets? Only on restart?
	for _, secret := range []*corev1.Secret{servicePrincipalSecretForBuild, servicePrincipalSecretForBuildahBuild, tokenSecretForBuildahCacheRegistry} {
		_, err = app.kubeutil.ApplySecret(buildNamespace, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app Application) createNewGitDeployKey(namespace, deployKey string, registration *v1.RadixRegistration) (*corev1.Secret, error) {
	knownHostsSecret, err := app.kubeutil.GetSecret(corev1.NamespaceDefault, "radix-known-hosts-git")
	if err != nil {
		log.Errorf("Failed to get known hosts secret. %v", err)
		return nil, err
	}

	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GitPrivateKeySecretName,
			Namespace: namespace,
			// Required when restoring with Velero. We only restore secrets with the "radix-app" label
			Labels: labels.ForApplicationName(registration.GetName()),
		},
		Data: map[string][]byte{
			defaults.GitPrivateKeySecretKey: []byte(deployKey),
			"known_hosts":                   knownHostsSecret.Data["known_hosts"],
		},
	}
	return &secret, nil
}

func (app Application) createNewServicePrincipalACRSecret(namespace, secretName string) (*corev1.Secret, error) {
	servicePrincipalSecret, err := app.kubeutil.GetSecret(corev1.NamespaceDefault, secretName)
	if err != nil {
		log.Errorf("Failed to get %s secret from default. %v", secretName, err)
		return nil, err
	}

	data := map[string][]byte{}
	for key := range servicePrincipalSecret.Data {
		data[key] = servicePrincipalSecret.Data[key]
	}

	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: data,
	}
	return &secret, nil
}

func (app Application) gitPrivateKeyExists(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}
	return len(strings.TrimSpace(string(secret.Data[defaults.GitPrivateKeySecretKey]))) > 0
}
func (app Application) createGitPublicKeyConfigMap(namespace string, key string, registration *v1.RadixRegistration) *corev1.ConfigMap {
	// Create a configmap with the public key
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GitPublicKeyConfigMapName,
			Namespace: namespace,
		}, Data: map[string]string{defaults.GitPublicKeyConfigMapKey: key}}

	return cm
}
