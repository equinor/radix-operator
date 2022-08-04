package kube

import (
	"context"
	"fmt"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	azureSecureStorageProvider                    = "azure"
	csiSecretProviderClassParameterUsePodIdentity = "usePodIdentity"
	csiSecretProviderClassParameterTenantId       = "tenantId"
	csiSecretProviderClassParameterCloudName      = "cloudName"
	csiSecretProviderClassParameterObjects        = "objects"
)

type stringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// SecretProviderClassParameterObject Object for SecretProviderClass parameters
type SecretProviderClassParameterObject struct {
	// Name. Name of the Azure Key Vault object
	Name string `yaml:"objectName"`
	// Type. Type of the Azure KeyVault object: secret, key, cert
	Type string `yaml:"objectType"`
	// Alias. Optional. Specify the filename of the object when written to disk. Defaults to objectName if not provided.
	Alias string `yaml:"objectAlias,omitempty"`
	// Version. Optional. object versions, default to the latest if empty
	Version string `yaml:"objectVersion,omitempty"`
	// Format. Optional. The format of the Azure Key Vault object, supported types are pem and pfx. objectFormat: pfx is only supported with objectType: secret and PKCS12 or ECC certificates. Default format for certificates is pem.
	Format string `yaml:"objectFormat,omitempty"`
	// Encoding. Optional. Setting object encoding to base64 and object format to pfx will fetch and write the base64 decoded pfx binary
	Encoding string `yaml:"objectEncoding,omitempty"`
}

// GetSecretProviderClass Gets secret provider class
func (kubeutil *Kube) GetSecretProviderClass(namespace string, className string) (*secretsstorev1.SecretProviderClass, error) {
	return kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.Background(), className, metav1.GetOptions{})
}

// CreateSecretProviderClass Creates secret provider class to namespace
func (kubeutil *Kube) CreateSecretProviderClass(namespace string, secretProviderClass *secretsstorev1.SecretProviderClass) (savedSecret *secretsstorev1.SecretProviderClass, err error) {
	log.Debugf("Create secret provider class %s in namespace %s", secretProviderClass.GetName(), namespace)
	return kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Create(context.TODO(), secretProviderClass, metav1.CreateOptions{})
}

// GetComponentSecretProviderClassName Gets unique name of the component secret storage class
func GetComponentSecretProviderClassName(radixDeploymentName, radixDeployComponentName string, radixSecretRefType radixv1.RadixSecretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a secret-ref they should not get
	// by naming component the same as secret-ref object
	hash := strings.ToLower(commonUtils.RandStringStrSeed(5, strings.ToLower(fmt.Sprintf("%s-%s-%s-%s",
		radixDeployComponentName, radixDeploymentName, radixSecretRefType, secretRefName))))
	return strings.ToLower(fmt.Sprintf("%s-%s-%s-%s", radixDeployComponentName, radixSecretRefType, secretRefName,
		hash))
}

//BuildAzureKeyVaultSecretProviderClass Build a SecretProviderClass for Azure Key vault secret-ref
func BuildAzureKeyVaultSecretProviderClass(tenantId string, appName string, radixDeploymentName string, radixDeployComponentName string, azureKeyVault radixv1.RadixAzureKeyVault) (*secretsstorev1.SecretProviderClass, error) {
	parameters, err := getAzureKeyVaultSecretProviderClassParameters(azureKeyVault, tenantId)
	if err != nil {
		return nil, err
	}
	secretProviderClass := buildSecretProviderClass(appName, radixDeploymentName, radixDeployComponentName,
		radixv1.RadixSecretRefTypeAzureKeyVault, azureKeyVault.Name)
	secretProviderClass.Spec.Parameters = parameters
	secretProviderClass.Spec.SecretObjects = getSecretProviderClassSecretObjects(radixDeployComponentName, radixDeploymentName, azureKeyVault)
	return secretProviderClass, nil
}

func getAzureKeyVaultSecretProviderClassParameters(radixAzureKeyVault radixv1.RadixAzureKeyVault, tenantId string) (map[string]string, error) {
	parameterMap := make(map[string]string)
	parameterMap[csiSecretProviderClassParameterUsePodIdentity] = "false"
	parameterMap[defaults.CsiSecretProviderClassParameterKeyVaultName] = radixAzureKeyVault.Name
	parameterMap[csiSecretProviderClassParameterTenantId] = tenantId
	parameterMap[csiSecretProviderClassParameterCloudName] = ""
	if len(radixAzureKeyVault.Items) == 0 {
		parameterMap[csiSecretProviderClassParameterObjects] = ""
		return parameterMap, nil
	}
	var parameterObject []SecretProviderClassParameterObject
	for _, item := range radixAzureKeyVault.Items {
		obj := SecretProviderClassParameterObject{
			Name:     item.Name,
			Alias:    commonUtils.StringUnPtr(item.Alias),
			Version:  commonUtils.StringUnPtr(item.Version),
			Format:   commonUtils.StringUnPtr(item.Format),
			Encoding: commonUtils.StringUnPtr(item.Encoding),
		}
		obj.Type = getObjectType(&item)
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

func getObjectType(item *radixv1.RadixAzureKeyVaultItem) string {
	if item.Type != nil {
		return string(*item.Type)
	}
	return string(radixv1.RadixAzureKeyVaultObjectTypeSecret)
}

func getSecretProviderClassSecretObjects(componentName, radixDeploymentName string, radixAzureKeyVault radixv1.RadixAzureKeyVault) []*secretsstorev1.SecretObject {
	var secretObjects []*secretsstorev1.SecretObject
	secretObjectMap := make(map[v1.SecretType]*secretsstorev1.SecretObject)
	for _, keyVaultItem := range radixAzureKeyVault.Items {
		kubeSecretType := GetSecretTypeForRadixAzureKeyVault(keyVaultItem.K8sSecretType)
		secretObject, existsSecretObject := secretObjectMap[kubeSecretType]
		if !existsSecretObject {
			secretObject = &secretsstorev1.SecretObject{
				SecretName: GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, radixAzureKeyVault.Name, kubeSecretType),
				Type:       string(kubeSecretType),
			}
			secretObjectMap[kubeSecretType] = secretObject
			secretObjects = append(secretObjects, secretObject)
		}
		data := secretsstorev1.SecretObjectData{
			Key:        GetSecretRefAzureKeyVaultItemDataKey(&keyVaultItem),
			ObjectName: getSecretRefAzureKeyVaultItemDataObjectName(&keyVaultItem),
		}
		secretObject.Data = append(secretObject.Data, &data)
	}
	return secretObjects
}

//GetSecretRefAzureKeyVaultItemDataKey Get item data key for the Azure Key vault secret-ref
func GetSecretRefAzureKeyVaultItemDataKey(keyVaultItem *radixv1.RadixAzureKeyVaultItem) string {
	if len(keyVaultItem.EnvVar) > 0 {
		return keyVaultItem.EnvVar
	}
	return fmt.Sprintf("%s--%s", keyVaultItem.Name, getObjectType(keyVaultItem))
}

func getSecretRefAzureKeyVaultItemDataObjectName(keyVaultItem *radixv1.RadixAzureKeyVaultItem) string {
	if keyVaultItem.Alias != nil && len(*keyVaultItem.Alias) != 0 {
		return *keyVaultItem.Alias
	}
	return keyVaultItem.Name
}

func buildSecretProviderClass(appName, radixDeploymentName, radixDeployComponentName string, radixSecretRefType radixv1.RadixSecretRefType, secretRefName string) *secretsstorev1.SecretProviderClass {
	secretRefName = strings.ToLower(secretRefName)
	className := GetComponentSecretProviderClassName(radixDeploymentName, radixDeployComponentName, radixSecretRefType, secretRefName)
	return &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: className,
			Labels: map[string]string{
				RadixAppLabel:           appName,
				RadixDeploymentLabel:    radixDeploymentName,
				RadixComponentLabel:     radixDeployComponentName,
				RadixSecretRefTypeLabel: string(radixSecretRefType),
				RadixSecretRefNameLabel: secretRefName,
			},
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			Provider: azureSecureStorageProvider,
		},
	}
}
