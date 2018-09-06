package onpush

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (cli *RadixOnPushHandler) deploy(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag string) error {
	appName := radixRegistration.Name
	log.Infof("Deploying app %s", appName)

	radixDeployments, err := createRadixDeployments(radixApplication, imageTag)
	if err != nil {
		return fmt.Errorf("Failed to create radix deployments objects for app %s", appName)
	}

	err = cli.applyRadixDeployments(radixRegistration, radixDeployments)
	if err != nil {
		return fmt.Errorf("Failed to apply radix deployments for app %s", appName)
	}
	log.Infof("App deployed %s", appName)

	return nil
}

func (cli *RadixOnPushHandler) applyRadixDeployments(radixRegistration *v1.RadixRegistration, radixDeployments []v1.RadixDeployment) error {
	for _, rd := range radixDeployments {
		err := applyEnvNamespace(cli.kubeclient, radixRegistration, rd)
		if err != nil {
			log.Warnf("Failed to create namespace: %s", rd.ObjectMeta.Namespace)
			return err
		}

		log.Infof("Apply radix deployment %s on env %s", rd.ObjectMeta.Name, rd.ObjectMeta.Namespace)
		_, err = cli.radixclient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Create(&rd)
		if err != nil {
			return err
		}
	}
	return nil
}

func applyEnvNamespace(kubeclient kubernetes.Interface, radixRegistration *v1.RadixRegistration, rd v1.RadixDeployment) error {
	namespaceName := rd.ObjectMeta.Namespace
	ownerRef := getOwnerRef(radixRegistration)

	log.Infof("Create namespace: %s", namespaceName)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            namespaceName,
			OwnerReferences: ownerRef,
		},
	}
	_, err := kubeclient.CoreV1().Namespaces().Create(&namespace)

	if errors.IsAlreadyExists(err) {
		log.Infof("Namespace already exist %s", namespaceName)
		return nil
	}

	return err
}

func createRadixDeployments(radixApplication *v1.RadixApplication, imageTag string) ([]v1.RadixDeployment, error) {
	radixDeployments := []v1.RadixDeployment{}
	for _, env := range radixApplication.Spec.Environments {
		radixComponents := getRadixComponentsForEnv(radixApplication, env.Name, imageTag)
		radixDeployment := createRadixDeployment(radixApplication.Name, env.Name, imageTag, radixComponents)
		radixDeployments = append(radixDeployments, radixDeployment)
	}

	return radixDeployments, nil
}

func createRadixDeployment(appName, env, imageTag string, components []v1.RadixDeployComponent) v1.RadixDeployment {
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", appName, imageTag),
			Namespace: fmt.Sprintf("%s-%s", appName, env),
			Labels: map[string]string{
				"radixApp": appName,
				"env":      env,
			},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     appName,
			Environment: env,
			Components:  components,
		},
	}
	return radixDeployment
}

func getRadixComponentsForEnv(radixApplication *v1.RadixApplication, env, imageTag string) []v1.RadixDeployComponent {
	appName := radixApplication.Name
	components := []v1.RadixDeployComponent{}
	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		_ = getEnvironmentVariables(appComponent, env)
		_ = getEnvironmentSecrets(appComponent)

		deployComponent := v1.RadixDeployComponent{
			Name:  componentName,
			Image: getImagePath(appName, componentName, imageTag),
		}
		components = append(components, deployComponent)
	}
	return components
}

func getEnvironmentVariables(component v1.RadixComponent, env string) error {
	for _, variable := range component.EnvironmentVariables {
		if variable.Environment != env {
			continue
		}

		// add env variable to result
	}
	return nil
}

func getEnvironmentSecrets(component v1.RadixComponent) error {
	return nil
}

func getOwnerRef(radixRegistration *v1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
			Kind:       "RadixDeployment",
			Name:       radixRegistration.Name,
			UID:        radixRegistration.UID,
			Controller: &trueVar,
		},
	}
}
