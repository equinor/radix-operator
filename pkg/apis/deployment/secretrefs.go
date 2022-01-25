package deployment

import (
	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func (deploy *Deployment) createSecretRefs(namespace string, component radixv1.RadixCommonDeployComponent) ([]string, error) {
	appName := deploy.registration.Name
	componentName := component.GetName()
	deploymentName := deploy.radixDeployment.GetName()
	var secretNames []string
	for _, radixAzureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		azureKeyVaultName := radixAzureKeyVault.Name

		credsSecret, err := deploy.getOrCreateAzureKeyVaultCredsSecret(namespace, appName, componentName, azureKeyVaultName)
		if err != nil {
			return nil, err
		}
		secretNames = append(secretNames, credsSecret.Name)

		className := kube.GetComponentSecretProviderClassName(componentName, deploymentName, radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVaultName)
		secretProviderClass, err := deploy.kubeutil.GetSecretProviderClass(namespace, className)
		if err == nil && secretProviderClass != nil {
			continue //SecretProviderClass already exists for this deployment and Azure Key vault
		}
		if !errors.IsNotFound(err) {
			return nil, err
		}
		secretProviderClass, err = deploy.createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace, appName, componentName, deploymentName, radixAzureKeyVault)
		if err != nil {
			return nil, err
		}
		if !isOwnerReference(credsSecret.ObjectMeta, secretProviderClass.ObjectMeta) {
			credsSecret.ObjectMeta.OwnerReferences = append(credsSecret.ObjectMeta.OwnerReferences, getOwnerReferenceOfSecretProviderClass(secretProviderClass))
			_, err = deploy.kubeutil.ApplySecret(namespace, credsSecret)
			if err != nil {
				return nil, err
			}
		}
	}
	return secretNames, nil
}

func (deploy *Deployment) createAzureKeyVaultSecretProviderClassForRadixDeployment(namespace string, appName string, componentName string, deploymentName string, azureKeyVault radixv1.RadixAzureKeyVault) (*secretsstorev1.SecretProviderClass, error) {
	parameters, err := getAzureKeyVaultSecretProviderClassParameters(azureKeyVault, deploy.tenantId)
	if err != nil {
		return nil, err
	}
	secretProviderClass := buildSecretProviderClass(appName, componentName, deploy.radixDeployment, radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVault.Name)
	secretProviderClass.Spec.Parameters = parameters
	secretProviderClass.Spec.SecretObjects = getSecretProviderClassSecretObjects(componentName, deploymentName, azureKeyVault)

	return deploy.kubeutil.CreateSecretProviderClass(namespace, secretProviderClass)
}

func getAzureKeyVaultSecretProviderClassParameters(radixAzureKeyVault radixv1.RadixAzureKeyVault, tenantId string) (map[string]string, error) {
	parameterMap := make(map[string]string)
	parameterMap[csiSecretProviderClassParameterUsePodIdentity] = "false"
	parameterMap[csiSecretProviderClassParameterKeyVaultName] = radixAzureKeyVault.Name
	parameterMap[csiSecretProviderClassParameterTenantId] = tenantId
	parameterMap[csiSecretProviderClassParameterCloudName] = ""
	if len(radixAzureKeyVault.Items) == 0 {
		parameterMap[csiSecretProviderClassParameterObjects] = ""
		return parameterMap, nil
	}
	var parameterObject []kube.SecretProviderClassParameterObject
	for _, item := range radixAzureKeyVault.Items {
		obj := kube.SecretProviderClassParameterObject{
			Name:     item.Name,
			Alias:    commonUtils.StringUnPtr(item.Alias),
			Version:  commonUtils.StringUnPtr(item.Version),
			Format:   commonUtils.StringUnPtr(item.Format),
			Encoding: commonUtils.StringUnPtr(item.Encoding),
		}
		if item.Type != nil {
			obj.Type = string(*item.Type)
		} else {
			obj.Type = string(radixv1.RadixAzureKeyVaultObjectTypeSecret)
		}
		parameterObject = append(parameterObject, obj)
	}
	parameterObjectArray := stringArray{}
	for _, keyVaultObject := range parameterObject {
		obj, err := yaml.Marshal(keyVaultObject)
		if err != nil {
			return nil, err
		}
		parameterObjectArray.Array = append(parameterObjectArray.Array, string(obj))
	}
	parameterObjectArrayString, err := yaml.Marshal(parameterObjectArray)
	if err != nil {
		return nil, err
	}
	parameterMap[csiSecretProviderClassParameterObjects] = string(parameterObjectArrayString)
	return parameterMap, nil
}

func getSecretProviderClassSecretObjects(componentName, radixDeploymentName string, radixAzureKeyVault radixv1.RadixAzureKeyVault) []*secretsstorev1.SecretObject {
	var secretObjects []*secretsstorev1.SecretObject
	secretObjectMap := make(map[kube.SecretType]*secretsstorev1.SecretObject)
	for _, keyVaultItem := range radixAzureKeyVault.Items {
		kubeSecretType := kube.GetSecretTypeForRadixAzureKeyVault(keyVaultItem.K8sSecretType)
		secretObject, existsSecretObject := secretObjectMap[kubeSecretType]
		if !existsSecretObject {
			secretObject = &secretsstorev1.SecretObject{
				SecretName: kube.GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, radixAzureKeyVault.Name, kubeSecretType),
				Type:       string(kubeSecretType),
			}
			secretObjectMap[kubeSecretType] = secretObject
			secretObjects = append(secretObjects, secretObject)
		}
		secretObject.Data = append(secretObject.Data, &secretsstorev1.SecretObjectData{
			ObjectName: keyVaultItem.Name,
			Key:        keyVaultItem.EnvVar,
		})
	}
	return secretObjects
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

func buildSecretProviderClass(appName, componentName string, radixDeployment *radixv1.RadixDeployment, radixSecretRefType radixv1.RadixSecretRefType, secretRefName string) *secretsstorev1.SecretProviderClass {
	className := kube.GetComponentSecretProviderClassName(componentName, radixDeployment.GetName(), radixSecretRefType, secretRefName)
	return &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: className,
			Labels: map[string]string{
				kube.RadixAppLabel:           appName,
				kube.RadixComponentLabel:     componentName,
				kube.RadixDeploymentLabel:    radixDeployment.GetName(),
				kube.RadixSecretRefTypeLabel: string(radixSecretRefType),
				kube.RadixSecretRefNameLabel: secretRefName,
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReferenceOfDeployment(radixDeployment),
			},
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			Provider: azureSecureStorageProvider,
		},
	}
}
