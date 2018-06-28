package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//CreateSecrets should provision required secrets in the specified environment
func (k *Kube) CreateSecrets(registration *radixv1.RadixRegistration, envName string) error {
	dockerSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find container registry credentials: %v", err)
	}
	ns := fmt.Sprintf("%s-%s", registration.Name, envName)
	dockerSecret.ResourceVersion = ""
	dockerSecret.Namespace = ns
	createdDockerSecret, err := k.kubeClient.CoreV1().Secrets(ns).Create(dockerSecret)
	if errors.IsAlreadyExists(err) {
		log.Infof("Secret object %s already exists in namespace %s, updating the object now", dockerSecret.Name, ns)
		updatedDockerSecret, err := k.kubeClient.CoreV1().Secrets(ns).Update(dockerSecret)
		if err != nil {
			return fmt.Errorf("Failed to update container registry credentials secret in %s: %v", ns, err)
		}
		log.Infof("Updated container registry credentials secret: %s in namespace %s", updatedDockerSecret.Name, ns)
	}
	if err != nil {
		return fmt.Errorf("Failed to create container registry credentials secret in %s: %v", ns, err)
	}
	log.Infof("Created container registry credentials secret: %s in namespace %s", createdDockerSecret.Name, ns)

	tlsSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("domain-ssl-cert-key", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find TLS certificate and key: %v", err)
	}
	tlsSecret.ResourceVersion = ""
	tlsSecret.Namespace = ns
	createdTlsSecret, err := k.kubeClient.CoreV1().Secrets(ns).Create(tlsSecret)
	if errors.IsAlreadyExists(err) {
		log.Infof("Secret object %s already exists in namespace %s, updating the object now", tlsSecret.Name, ns)
		updatedTlsSecret, err := k.kubeClient.CoreV1().Secrets(ns).Update(tlsSecret)
		if err != nil {
			return fmt.Errorf("Failed to update TLS certificate and key secret in %s: %v", ns, err)
		}
		log.Infof("Updated TLS certificate and key secret: %s in namespace %s", updatedTlsSecret.Name, ns)
	}
	if err != nil {
		return fmt.Errorf("Failed to create TLS certificate and key secret in %s: %v", ns, err)
	}
	log.Infof("Created TLS certificate and key secret: %s in namespace %s", createdTlsSecret.Name, ns)

	return nil
}

//CreateEnvironment creates a namespace with RadixRegistration as owner
func (k *Kube) CreateEnvironment(registration *radixv1.RadixRegistration, envName string) error {
	trueVar := true
	name := fmt.Sprintf("%s-%s", registration.Name, envName)
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"radixApp": registration.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1",
					Kind:       "RadixRegistration",
					Name:       registration.Name,
					UID:        registration.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: v1.NamespaceSpec{},
	}

	_, err := k.kubeClient.CoreV1().Namespaces().Create(ns)
	if errors.IsAlreadyExists(err) {
		log.Infof("Namespace %s already exists", name)
		return nil
	}

	if err != nil {
		log.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}
	log.Infof("Created namespace %s", name)
	return nil
}
