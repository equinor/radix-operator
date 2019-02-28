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
		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(appComponent, env)

		variables := make(map[string]string)
		monitoring := false
		var resources v1.ResourceRequirements

		// Will later be overriden by default replicas if not set specifically
		replicas := 0

		if environmentSpecificConfig != nil {
			replicas = environmentSpecificConfig.Replicas
			variables = environmentSpecificConfig.Variables
			monitoring = environmentSpecificConfig.Monitoring
			resources = environmentSpecificConfig.Resources
		}

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                utils.GetImagePath(containerRegistry, appName, componentName, imageTag),
			Replicas:             replicas,
			Public:               appComponent.Public,
			Ports:                appComponent.Ports,
			Secrets:              appComponent.Secrets,
			EnvironmentVariables: variables, // todo: use single EnvVars instead
			DNSAppAlias:          env == dnsAppAlias.Environment && componentName == dnsAppAlias.Component,
			Monitoring:           monitoring,
			Resources:            resources,
		}

		components = append(components, deployComponent)
	}
	return components
}

func getEnvironmentSpecificConfigForComponent(component v1.RadixComponent, env string) *v1.RadixEnvironmentConfig {
	if component.EnvironmentConfig == nil {
		return nil
	}

	for _, environment := range component.EnvironmentConfig {
		if environment.Environment == env {
			return &environment
		}
	}
	return nil
}
