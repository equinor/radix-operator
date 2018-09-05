package onpush

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func deploy(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag string) error {
	radixDeployments, err := getRadixDeployments(radixRegistration, radixApplication, imageTag)
	if err != nil {
		return fmt.Errorf("Failed to create radix deployments objects for app %s", radixRegistration.Name)
	}

	applyRadixDeployments(radixDeployments)

	return nil
}

func applyRadixDeployments(radixDeployments []v1.RadixDeployment) {
	for _, rd := range radixDeployments {
		fmt.Printf("appling radix deployment %s on env %s", rd.Name, rd.Namespace)
	}
}

func getRadixDeployments(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag string) ([]v1.RadixDeployment, error) {
	radixDeployments := []v1.RadixDeployment{}
	for _, env := range radixApplication.Spec.Environments {
		radixComponents := getRadixComponentsForEnv(radixApplication, env.Name, imageTag)
		radixDeployment := createRadixDeployment(radixRegistration, env.Name, radixComponents)
		radixDeployments = append(radixDeployments, radixDeployment)
	}

	return radixDeployments, nil
}

func createRadixDeployment(radixRegistration *v1.RadixRegistration, env string, components []v1.RadixDeployComponent) v1.RadixDeployment {
	ownerRef := getOwnerRef(radixRegistration)
	appName := radixRegistration.Name
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: fmt.Sprintf("%s-%s", appName, env),
			Labels: map[string]string{
				"radixApp": appName,
				"env":      env,
			},
			OwnerReferences: ownerRef,
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
		radixEnvironmentVariables := getEnvironmentVariables(radixApplication, appComponent, env)
		log.Printf("got radix env, %s", radixEnvironmentVariables)

		deployComponent := v1.RadixDeployComponent{
			Name:  componentName,
			Image: getImagePath(appName, componentName, imageTag),
		}
		components = append(components, deployComponent)
	}
	return components
}

func getEnvironmentVariables(radixApplication *v1.RadixApplication, component v1.RadixComponent, env string) []v1.RadixDeployComponent {
	components := []v1.RadixDeployComponent{}
	return components
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
