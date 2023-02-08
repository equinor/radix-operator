package application

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	publicKeyExists, err := app.gitPublicKeyExists(namespace)
	if err != nil {
		return err
	}
	if publicKeyExists {
		deployKey, err := utils.GenerateDeployKey()
		if err != nil {
			return err
		}
		configMap := app.createGitPublicKeyConfigMap(namespace, deployKey.PublicKey, radixRegistration)
		err = app.kubeutil.ApplyConfigMap(configMap)
		if err != nil {
			return err
		}
		secret, err := app.createNewGitDeployKey(namespace, deployKey.PrivateKey, radixRegistration)
		_, err = app.kubeutil.ApplySecret(namespace, secret)
		if err != nil {
			return err
		}
		return nil
	}

	// TODO: apply ownerReference to secret and cm

	/*deployKey, err = utils.GenerateDeployKey()
	if err != nil && !k8errors.IsNotFound(err) {
		return err
	}
	if k8errors.IsNotFound(err) && radixRegistration.Spec.DeployKey == "" {
		return fmt.Errorf("secret git-ssh-keys not found in namespace %s, and no deploy key is set in RadixRegistration %s", namespace, radixRegistration.Name)
	}

	if k8errors.IsNotFound(err) && radixRegistration.Spec.DeployKey != "" {
		secret, err := app.createNewGitDeployKey(namespace, radixRegistration.Spec.DeployKey)
		if err != nil {
			return err
		}
		_, err = app.kubeutil.ApplySecret(namespace, secret)
		if err != nil {
			return err
		}
		return nil
	}

	if err == nil && radixRegistration.Spec.DeployKey != "" {
		log.Warnf("secret git-ssh-keys already exists in namespace %s, and deploy key is set in RadixRegistration %s. Unexpected state, deleting deploy key from RadixRegistration", namespace, radixRegistration.Name)
	}
	*/
	return err
}

// Case 2: Hvis secreten er tom, og radixregistration.spec.deploykey er satt, så opprettes en ny secret med deploykey
// Case 1: Hvis secreten er tom, og radixregistration.spec.deploykey er ikke satt, så panic
// Case 4: Hvis secreten er satt, og radixregistration.spec.deploykey er satt, så printes en warning og secret slettes fra RR
// Case 3: Hvis secreten er satt, og radixregistration.spec.deploykey er ikke satt, så skjer ingenting

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
			Name:            "git-ssh-keys",
			Namespace:       namespace,
			OwnerReferences: GetOwnerReferenceOfRegistration(registration),
		},
		Data: map[string][]byte{
			"id_rsa":      []byte(deployKey),
			"known_hosts": knownHostsSecret.Data["known_hosts"],
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
