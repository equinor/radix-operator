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
		var replicas *int

		if environmentSpecificConfig != nil {
			replicas = environmentSpecificConfig.Replicas
			variables = environmentSpecificConfig.Variables
			monitoring = environmentSpecificConfig.Monitoring
			resources = environmentSpecificConfig.Resources
		}

		externalAlias := GetExternalDNSAliasForComponentEnvironment(radixApplication, componentName, env)

		var image string
		if appComponent.Image != "" {
			// Use public image in deployment
			image = appComponent.Image
		} else {
			image = utils.GetImagePath(containerRegistry, appName, componentName, imageTag)
		}

		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Image:                image,
			Replicas:             replicas,
			Public:               false,
			PublicPort:           getPublicPortFromAppComponent(appComponent),
			Ports:                appComponent.Ports,
			Secrets:              appComponent.Secrets,
			IngressConfiguration: appComponent.IngressConfiguration,
			EnvironmentVariables: variables, // todo: use single EnvVars instead
			DNSAppAlias:          IsDNSAppAlias(env, componentName, dnsAppAlias),
			DNSExternalAlias:     externalAlias,
			Monitoring:           monitoring,
			Resources:            resources,
		}

		components = append(components, deployComponent)
	}
	return components
}

// IsDNSAppAlias Checks if environment and component represents the DNS app alias
func IsDNSAppAlias(env, componentName string, dnsAppAlias v1.AppAlias) bool {
	return env == dnsAppAlias.Environment && componentName == dnsAppAlias.Component
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

func getPublicPortFromAppComponent(appComponent v1.RadixComponent) string {
	if appComponent.PublicPort == "" && appComponent.Public {
		return appComponent.Ports[0].Name
	}

	return appComponent.PublicPort
}

// GetExternalDNSAliasForComponentEnvironment Gets external DNS alias
func GetExternalDNSAliasForComponentEnvironment(radixApplication *v1.RadixApplication, component, env string) []string {
	dnsExternalAlias := make([]string, 0)

	for _, externalAlias := range radixApplication.Spec.DNSExternalAlias {
		if externalAlias.Component == component && externalAlias.Environment == env {
			dnsExternalAlias = append(dnsExternalAlias, externalAlias.Alias)
		}
	}

	return dnsExternalAlias
}
