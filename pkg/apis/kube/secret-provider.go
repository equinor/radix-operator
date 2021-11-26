package kube

import (
	"context"
	"encoding/json"
	"fmt"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// SecretProviderClassObject Object for SecretProviderClass parameters
type SecretProviderClassObject struct {
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
func (kubeutil *Kube) GetSecretProviderClass(namespace string, componentName string, radixSecretRefType string, radixSecretRefName string) (*secretsstorev1.SecretProviderClass, error) {
	classList, err := kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: GetLabelSelectorForSecretRefObject(componentName, radixSecretRefType, radixSecretRefName)})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(classList.Items) == 0 {
		return nil, nil
	}
	if len(classList.Items) == 0 {
		return nil, nil
	}
	if len(classList.Items) > 1 {
		return nil, fmt.Errorf("found multiple SecretProviderClass for the same RadixSecretRef")
	}
	return &classList.Items[0], err
}

// ListSecretProviderClass Gets secret provider classes for the component
func (kubeutil *Kube) ListSecretProviderClass(namespace string, componentName string) ([]secretsstorev1.SecretProviderClass, error) {
	classList, err := kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: GetLabelSelectorForAllSecretRefObjects(componentName)})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return classList.Items, err
}

// GetLabelSelectorForSecretRefObject Get label selector for secret-ref object (secret, secret provider class, etc.)
func GetLabelSelectorForSecretRefObject(componentName string, radixSecretRefType string, radixSecretRefName string) string {
	return fmt.Sprintf("%s=%s, %s=%s, %s=%s", RadixComponentLabel, componentName, RadixSecretRefTypeLabel, radixSecretRefType, RadixSecretRefNameLabel, radixSecretRefName)
}

// GetLabelSelectorForAllSecretRefObjects Get label selector for all secret-ref objects (secret, secret provider class, etc.)
func GetLabelSelectorForAllSecretRefObjects(componentName string) string {
	return fmt.Sprintf("%s=%s, %s", RadixComponentLabel, componentName, RadixSecretRefTypeLabel)
}

// ApplySecretProviderClass Creates or updates secret provider class to namespace
func (kubeutil *Kube) ApplySecretProviderClass(namespace string, secretProviderClass *secretsstorev1.SecretProviderClass) (savedSecret *secretsstorev1.SecretProviderClass, err error) {
	className := secretProviderClass.GetName()
	log.Debugf("Applies secret provider class %s in namespace %s", className, namespace)

	componentName := secretProviderClass.Labels[RadixComponentLabel]
	radixSecretRefType := secretProviderClass.Labels[RadixSecretRefTypeLabel]
	radixSecretRefName := secretProviderClass.Labels[RadixSecretRefNameLabel]
	oldClass, err := kubeutil.GetSecretProviderClass(namespace, componentName, radixSecretRefType, radixSecretRefName)
	secretProviderClasses := kubeutil.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace)
	if oldClass == nil || (err != nil && errors.IsNotFound(err)) {
		savedClass, err := secretProviderClasses.Create(context.TODO(), secretProviderClass, metav1.CreateOptions{})
		return savedClass, err
	} else if err != nil {
		return nil, fmt.Errorf("failed to get secret provider class object: %v", err)
	}

	oldClassJSON, err := json.Marshal(oldClass)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal old secret object: %v", err)
	}

	// Avoid unnecessary patching
	newClass := oldClass.DeepCopy()
	newClass.ObjectMeta.Labels = secretProviderClass.ObjectMeta.Labels
	newClass.ObjectMeta.Annotations = secretProviderClass.ObjectMeta.Annotations
	newClass.ObjectMeta.OwnerReferences = secretProviderClass.ObjectMeta.OwnerReferences
	newClass.Spec = secretProviderClass.Spec

	newClassJSON, err := json.Marshal(newClass)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new secret provider class object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClassJSON, newClassJSON, secretsstorev1.SecretProviderClass{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch secret provider class objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		// Will perform update as patching not properly remove secret provider class data entries
		patchedClass, err := secretProviderClasses.Update(context.TODO(), newClass, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("filed to update secret provider class object: %v", err)
		}

		log.Debugf("Updated secret provider class: %s ", patchedClass.Name)
		return patchedClass, nil

	}

	log.Debugf("No need to patch secret provider class: %s ", className)
	return oldClass, nil
}

// DeleteChangedSecretProviderClass Deletes a role in a namespace
func (kubeutil *Kube) DeleteChangedSecretProviderClass(namespace string, componentName string, secretRef radixv1.RadixSecretRef) error {
	//TODO
	//class := secretsstorev1.SecretProviderClass{}
	////_, err := kubeutil.kubeClient.CoreV1().Namespaces(namespace).Delete( GetKV)
	//if err != nil && errors.IsNotFound(err) {
	//	return nil
	//} else if err != nil {
	//	return fmt.Errorf("Failed to get role object: %v", err)
	//}
	//err = kubeutil.kubeClient.RbacV1().Roles(namespace).Delete(context.TODO(), sc, metav1.DeleteOptions{})
	//if err != nil {
	//	return fmt.Errorf("Failed to delete role object: %v", err)
	//}
	return nil
}
