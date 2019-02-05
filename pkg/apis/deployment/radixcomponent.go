package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

func getRadixComponentsForEnv(radixApplication *v1.RadixApplication, containerRegistry, env, imageTag string) []v1.RadixDeployComponent {
	appName := radixApplication.Name
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	components := []v1.RadixDeployComponent{}

	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		variables := getEnvironmentVariables(appComponent, env)

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                utils.GetImagePath(containerRegistry, appName, componentName, imageTag),
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
