package onpush

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	log "github.com/Sirupsen/logrus"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Deploy Handles deploy step of the pipeline
func (cli *RadixOnPushHandler) Deploy(jobName string, radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	appName := radixRegistration.Name
	containerRegistry, err := cli.kubeutil.GetContainerRegistry()
	if err != nil {
		return nil, err
	}

	log.Infof("Deploying app %s", appName)

	radixDeployments, err := createRadixDeployments(radixApplication, containerRegistry, jobName, imageTag, branch, commitID, targetEnvs)
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

// TODO : Move this into Deployment domain/package
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

// TODO : Move this into Deployment domain/package
func (cli *RadixOnPushHandler) applyEnvNamespaces(radixRegistration *v1.RadixRegistration, targetEnvs map[string]bool) error {
	for env := range targetEnvs {
		namespaceName := utils.GetEnvironmentNamespace(radixRegistration.Name, env)
		ownerRef := kube.GetOwnerReferenceOfRegistration(radixRegistration)
		labels := map[string]string{
			"sync":                  "cluster-wildcard-tls-cert",
			"cluster-wildcard-sync": "cluster-wildcard-tls-cert",
			"app-wildcard-sync":     "app-wildcard-tls-cert",
			kube.RadixAppLabel:      radixRegistration.Name,
			kube.RadixEnvLabel:      env,
		}

		err := cli.kubeutil.ApplyNamespace(namespaceName, labels, ownerRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO : Move this into Deployment domain/package
func createRadixDeployments(radixApplication *v1.RadixApplication, containerRegistry, jobName, imageTag, branch, commitID string, targetEnvs map[string]bool) ([]v1.RadixDeployment, error) {
	radixDeployments := []v1.RadixDeployment{}
	for _, env := range radixApplication.Spec.Environments {
		if _, contains := targetEnvs[env.Name]; !contains {
			continue
		}

		if !targetEnvs[env.Name] {
			// Target environment exists in config but should not be built
			continue
		}

		radixComponents := getRadixComponentsForEnv(radixApplication, containerRegistry, env.Name, imageTag)
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
			Namespace: utils.GetEnvironmentNamespace(appName, env),
			Labels: map[string]string{
				"radixApp":             appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:     appName,
				kube.RadixEnvLabel:     env,
				kube.RadixBranchLabel:  branch,
				kube.RadixCommitLabel:  commitID,
				kube.RadixJobNameLabel: jobName,
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

func getRadixComponentsForEnv(radixApplication *v1.RadixApplication, containerRegistry, env, imageTag string) []v1.RadixDeployComponent {
	appName := radixApplication.Name
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	components := []v1.RadixDeployComponent{}

	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		variables := getEnvironmentVariables(appComponent, env)

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                getImagePath(containerRegistry, appName, componentName, imageTag),
			Replicas:             appComponent.Replicas,
			Public:               appComponent.Public,
			Ports:                appComponent.Ports,
			Secrets:              appComponent.Secrets,
			EnvironmentVariables: variables, // todo: use single EnvVars instead
			DNSAppAlias:          env == dnsAppAlias.Environment && componentName == dnsAppAlias.Component,
			Monitoring:           appComponent.Monitoring,
			Resources:            appComponent.Resources,
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
