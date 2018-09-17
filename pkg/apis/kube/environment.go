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
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})
	ns := fmt.Sprintf("%s-%s", registration.Name, envName)

	err := k.createDockerSecret(registration, ns)
	if err != nil {
		return err
	}

	return k.createTlsSecret(registration, ns)

}

func (k *Kube) createTlsSecret(registration *radixv1.RadixRegistration, ns string) error {
	tlsSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("domain-ssl-cert-key", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find TLS certificate and key: %v", err)
	}
	tlsSecret.ResourceVersion = ""
	tlsSecret.Namespace = ns
	saveTLSSecret, err := k.ApplySecret(ns, tlsSecret)
	if err != nil {
		return fmt.Errorf("Failed to save TLS certificate and key secret in %s: %v", ns, err)
	}
	logger.Infof("Created TLS certificate and key secret: %s in namespace %s", saveTLSSecret.Name, ns)
	return nil
}

func (k *Kube) createDockerSecret(registration *radixv1.RadixRegistration, ns string) error {
	dockerSecret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find container registry credentials: %v", err)
	}
	dockerSecret.ResourceVersion = ""
	dockerSecret.Namespace = ns
	dockerSecret.UID = ""
	saveDockerSecret, err := k.ApplySecret(ns, dockerSecret)
	if err != nil {
		return fmt.Errorf("Failed to create container registry credentials secret in %s: %v", ns, err)
	}
	logger.Infof("Created container registry credentials secret: %s in namespace %s", saveDockerSecret.Name, ns)
	return nil
}

//CreateEnvironment creates a namespace with RadixRegistration as owner
func (k *Kube) CreateEnvironment(registration *radixv1.RadixRegistration, envName string) error {
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})

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
		logger.Infof("Namespace %s already exists", name)
		return nil
	}

	if err != nil {
		logger.Errorf("Failed to create namespace %s: %v", name, err)
		return err
	}
	logger.Infof("Created namespace %s", name)
	return nil
}
