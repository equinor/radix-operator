package kube

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"strings"
)

// StringArray ...
type StringArray struct {
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
	// Version. Optional. object versions, default to latest if empty
	Version string `yaml:"objectVersion,omitempty"`
	// Format. Optional. The format of the Azure Key Vault object, supported types are pem and pfx. objectFormat: pfx is only supported with objectType: secret and PKCS12 or ECC certificates. Default format for certificates is pem.
	Format string `yaml:"objectFormat,omitempty"`
	// Encoding. Optional. Setting object encoding to base64 and object format to pfx will fetch and write the base64 decoded pfx binary
	Encoding string `yaml:"objectEncoding,omitempty"`
}

// GetSecretProviderClass Gets secret provider class
func (kubeutil *Kube) GetSecretProviderClass(namespace string, className string) (*secretsstorev1.SecretProviderClass, error) {
	class, err := kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Get(context.Background(), className, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return class, err
}

// ListSecretProviderClass Gets secret provider classes for the component
func (kubeutil *Kube) ListSecretProviderClass(namespace string, componentName string) ([]secretsstorev1.SecretProviderClass, error) {
	classList, err := kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: GetAllSecretRefObjectsLabelSelector(componentName)})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return classList.Items, err
}

// GetSecretRefObjectLabelSelector Get label selector for secret-ref object (secret, secret provider class, etc.)
func GetSecretRefObjectLabelSelector(componentName, radixDeploymentName string, radixSecretRefType radixv1.RadixSecretRefType, radixSecretRefName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s=%s, %s=%s", RadixComponentLabel, componentName, RadixDeploymentLabel, radixDeploymentName, RadixSecretRefTypeLabel, string(radixSecretRefType), RadixSecretRefNameLabel, radixSecretRefName)
}

// GetAllSecretRefObjectsLabelSelector Get label selector for all secret-ref objects (secret, secret provider class, etc.)
func GetAllSecretRefObjectsLabelSelector(componentName string) string {
	return fmt.Sprintf("%s=%s, %s", RadixComponentLabel, componentName, RadixSecretRefTypeLabel)
}

// CreateSecretProviderClass Creates secret provider class to namespace
func (kubeutil *Kube) CreateSecretProviderClass(namespace string, secretProviderClass *secretsstorev1.SecretProviderClass) (savedSecret *secretsstorev1.SecretProviderClass, err error) {
	log.Debugf("Create secret provider class %s in namespace %s", secretProviderClass.GetName(), namespace)
	return kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Create(context.TODO(), secretProviderClass, metav1.CreateOptions{})
}

// GetComponentSecretProviderClassName Gets unique name of the component secret storage class
func GetComponentSecretProviderClassName(componentName, radixDeploymentName string, radixSecretRefType radixv1.RadixSecretRefType, secretRefName string) string {
	// include a hash so that users cannot get access to a secret-ref they should not get
	// by naming component the same as secret-ref object
	hash := strings.ToLower(utils.RandStringStrSeed(5, fmt.Sprintf("%s-%s-%s-%s", componentName, radixDeploymentName, radixSecretRefType, secretRefName)))
	return fmt.Sprintf("%s-%s-%s-%s", componentName, radixSecretRefType, secretRefName, hash)
}