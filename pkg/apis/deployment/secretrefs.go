package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func (deploy *Deployment) createSecretRefs(namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent) ([]string, error) {
	appName := deploy.registration.Name
	radixDeployComponentName := radixDeployComponent.GetName()
	radixDeploymentName := deploy.radixDeployment.GetName()
	var secretNames []string
	for _, radixAzureKeyVault := range radixDeployComponent.GetSecretRefs().AzureKeyVaults {
		azureKeyVaultName := radixAzureKeyVault.Name

		className := kube.GetComponentSecretProviderClassName(radixDeploymentName, radixDeployComponentName, radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVaultName)
		secretProviderClass, err := deploy.kubeutil.GetSecretProviderClass(namespace, className)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		var credsSecret *v1.Secret
		useAzureIdentity := radixAzureKeyVault.UseAzureIdentity != nil && *radixAzureKeyVault.UseAzureIdentity
		if !useAzureIdentity {
			credsSecret, err := deploy.getOrCreateAzureKeyVaultCredsSecret(namespace, appName, radixDeployComponentName, azureKeyVaultName)
			if err != nil {
				return nil, err
			}
			secretNames = append(secretNames, credsSecret.Name)
		}
		if err == nil && secretProviderClass != nil {
			continue // SecretProviderClass already exists for this deployment and Azure Key vault
		}
		secretProviderClass, err = deploy.createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace, appName, radixDeployComponentName, radixAzureKeyVault)
		if err != nil {
			return nil, err
		}
		if !useAzureIdentity && credsSecret != nil && !isOwnerReference(credsSecret.ObjectMeta, secretProviderClass.ObjectMeta) {
			credsSecret.ObjectMeta.OwnerReferences = append(credsSecret.ObjectMeta.OwnerReferences, getOwnerReferenceOfSecretProviderClass(secretProviderClass))
			_, err = deploy.kubeutil.ApplySecret(namespace, credsSecret)
			if err != nil {
				return nil, err
			}
		}
	}
	return secretNames, nil
}

func (deploy *Deployment) createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace string, appName string, radixDeployComponentName string, azureKeyVault radixv1.RadixAzureKeyVault) (*secretsstorev1.SecretProviderClass, error) {
	radixDeploymentName := deploy.radixDeployment.GetName()
	tenantId := deploy.tenantId
	identity := getIdentityFromRadixCommonDeployComponent(deploy, radixDeployComponentName)
	secretProviderClass, err := kube.BuildAzureKeyVaultSecretProviderClass(tenantId, appName, radixDeploymentName, radixDeployComponentName, azureKeyVault, identity)
	if err != nil {
		return nil, err
	}
	secretProviderClass.OwnerReferences = []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}
	return deploy.kubeutil.CreateSecretProviderClass(namespace, secretProviderClass)
}

func getIdentityFromRadixCommonDeployComponent(deploy *Deployment, radixDeployComponentName string) *radixv1.Identity {
	if radixDeployComponent := deploy.radixDeployment.GetComponentByName(radixDeployComponentName); radixDeployComponent != nil {
		return radixDeployComponent.GetIdentity()
	}
	if radixJobDeployComponent := deploy.radixDeployment.GetJobComponentByName(radixDeployComponentName); radixJobDeployComponent != nil {
		return radixJobDeployComponent.GetIdentity()
	}
	return nil
}

func (deploy *Deployment) getOrCreateAzureKeyVaultCredsSecret(namespace, appName, componentName, azKeyVaultName string) (*v1.Secret, error) {
	secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(componentName, azKeyVaultName)
	secret, err := deploy.kubeutil.GetSecret(namespace, secretName)
	if err != nil {
		if errors.IsNotFound(err) {
			secret = buildAzureKeyVaultCredentialsSecret(appName, componentName, secretName, azKeyVaultName)
			return deploy.kubeutil.ApplySecret(namespace, secret)
		}
		return nil, err
	}
	return secret, nil
}
