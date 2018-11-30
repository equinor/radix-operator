package onpush

import (
	"fmt"

	"github.com/statoil/radix-operator/pkg/apis/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Deploy Handles deploy step of the pipeline
func (cli *RadixOnPushHandler) Deploy(jobName string, radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	appName := radixRegistration.Name
	log.Infof("Deploying app %s", appName)

	radixDeployments, err := createRadixDeployments(radixApplication, jobName, imageTag, branch, commitID, targetEnvs)
	if err != nil {
		return nil, fmt.Errorf("Failed to create radix deployments objects for app %s. %v", appName, err)
	}

	err = cli.applyEnvNamespaces(radixRegistration, targetEnvs)
	if err != nil {
		log.Errorf("Failed to create namespaces for app environments %s. %v", radixRegistration.Name, err)
		return nil, err
	}

	err = cli.applyRadixDeployments(radixRegistration, radixDeployments)
	if err != nil {
		return nil, fmt.Errorf("Failed to apply radix deployments for app %s. %v", appName, err)
	}
	log.Infof("App deployed %s", appName)

	return radixDeployments, nil
}

func (cli *RadixOnPushHandler) applyRadixDeployments(radixRegistration *v1.RadixRegistration, radixDeployments []v1.RadixDeployment) error {
	for _, rd := range radixDeployments {
		log.Infof("Apply radix deployment %s on env %s", rd.ObjectMeta.Name, rd.ObjectMeta.Namespace)
		_, err := cli.radixclient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Create(&rd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *RadixOnPushHandler) applyEnvNamespaces(radixRegistration *v1.RadixRegistration, targetEnvs map[string]bool) error {
	for env := range targetEnvs {
		namespaceName := fmt.Sprintf("%s-%s", radixRegistration.Name, env)
		ownerRef := getOwnerRef(radixRegistration)
		labels := map[string]string{
			"sync":      "cluster-wildcard-tls-cert",
			"radix-app": radixRegistration.Name,
			"radix-env": env,
		}

		err := cli.kubeutil.ApplyNamespace(namespaceName, labels, ownerRef)
		if err != nil {
			return err
		}
	}

	return nil
}

func createRadixDeployments(radixApplication *v1.RadixApplication, jobName, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	radixDeployments := []v1.RadixDeployment{}
	for _, env := range radixApplication.Spec.Environments {
		if _, contains := targetEnvs[env.Name]; !contains {
			continue
		}

		if !targetEnvs[env.Name] {
			// Target environment exists in config but should not be built
			continue
		}

		radixComponents := getRadixComponentsForEnv(radixApplication, env.Name, imageTag)
		radixDeployment := createRadixDeployment(radixApplication.Name, env.Name, jobName, imageTag, branch, commitID, radixComponents)
		radixDeployments = append(radixDeployments, radixDeployment)
	}

	return radixDeployments, nil
}

func createRadixDeployment(appName, env, jobName, imageTag, branch, commitID string, components []v1.RadixDeployComponent) v1.RadixDeployment {
	deployName := utils.GetDeploymentName(appName, env, imageTag)
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: fmt.Sprintf("%s-%s", appName, env),
			Labels: map[string]string{
				"radixApp":       appName, // For backwards compatibility. Remove when cluster is migrated
				"radix-app":      appName,
				"radix-env":      env,
				"radix-branch":   branch,
				"radix-commit":   commitID,
				"radix-job-name": jobName,
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
		variables := getEnvironmentVariables(appComponent, env)

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                getImagePath(appName, componentName, imageTag),
			Replicas:             appComponent.Replicas,
			Public:               appComponent.Public,
			Ports:                appComponent.Ports,
			Secrets:              appComponent.Secrets,
			EnvironmentVariables: variables, // todo: use single EnvVars instead
		}
		components = append(components, deployComponent)
	}
	return components
}

func getEnvironmentVariables(component v1.RadixComponent, env string) v1.EnvVarsMap {
	if component.EnvironmentVariables == nil {
		return v1.EnvVarsMap{}
	}

	for _, variables := range component.EnvironmentVariables {
		if variables.Environment == env {
			return variables.Variables
		}
	}
	return v1.EnvVarsMap{}
}

func getOwnerRef(radixRegistration *v1.RadixRegistration) []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
			Name:       radixRegistration.Name,
			UID:        radixRegistration.UID,
			Controller: &trueVar,
		},
	}
}
