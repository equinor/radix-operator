package application

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
		log.Warnf("Failed to apply service principle acr secret (%s) to namespace %s", defaults.AzureACRServicePrincipleSecretName, buildNamespace)
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
	privateKeyExists := app.gitPrivateKeyExists(secret) // checking if private key exists
	if !privateKeyExists {
		var deployKey *utils.DeployKey
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
			deployKey = &utils.DeployKey{
				PrivateKey: radixRegistration.Spec.DeployKey,
				PublicKey:  radixRegistration.Spec.DeployKeyPublic,
			}
			if err != nil {
				return err
			}
		}
		newCm := app.createGitPublicKeyConfigMap(namespace, deployKey.PublicKey, radixRegistration)
		_, err = app.kubeutil.CreateConfigMap(namespace, newCm)
		if k8serrors.IsAlreadyExists(err) {
			existingCm, err := app.kubeutil.GetConfigMap(namespace, defaults.GitPublicKeyConfigMapName)
			if err != nil {
				return err
			}
			err = app.kubeutil.ApplyConfigMap(namespace, existingCm, newCm)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		// also create corresponding secret with private key
		secret, err := app.createNewGitDeployKey(namespace, deployKey.PrivateKey, radixRegistration)
		if err != nil {
			return err
		}
		_, err = app.kubeutil.ApplySecret(namespace, secret)
		if err != nil {
			return err
		}
		return nil
	}

	// apply ownerReference to secret
	secret.OwnerReferences = GetOwnerReferenceOfRegistration(radixRegistration)
	_, err = app.kubeutil.ApplySecret(namespace, secret)

	return err
}

func (app Application) applyServicePrincipalACRSecretToBuildNamespace(buildNamespace string) error {
	servicePrincipalSecretForBuild, err := app.createNewServicePrincipalACRSecret(buildNamespace)
	if err != nil {
		return err
	}

	_, err = app.kubeutil.ApplySecret(buildNamespace, servicePrincipalSecretForBuild)
	return err
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
			Name:            defaults.GitPrivateKeySecretName,
			Namespace:       namespace,
			OwnerReferences: GetOwnerReferenceOfRegistration(registration),
		},
		Data: map[string][]byte{
			defaults.GitPrivateKeySecretKey: []byte(deployKey),
			"known_hosts":                   knownHostsSecret.Data["known_hosts"],
		},
	}
	return &secret, nil
}

func (app Application) createNewServicePrincipalACRSecret(namespace string) (*corev1.Secret, error) {
	servicePrincipalSecret, err := app.kubeutil.GetSecret(corev1.NamespaceDefault, defaults.AzureACRServicePrincipleSecretName)
	if err != nil {
		log.Errorf("Failed to get %s secret from default. %v", defaults.AzureACRServicePrincipleSecretName, err)
		return nil, err
	}
	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.AzureACRServicePrincipleSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"sp_credentials.json": servicePrincipalSecret.Data["sp_credentials.json"],
		},
	}
	return &secret, nil
}
