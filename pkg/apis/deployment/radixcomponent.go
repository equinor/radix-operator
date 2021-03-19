package deployment

import (
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func getRadixComponentsForEnv(radixApplication *v1.RadixApplication, env string, componentImages map[string]pipeline.ComponentImage) []v1.RadixDeployComponent {
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	components := []v1.RadixDeployComponent{}

	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		componentImage := componentImages[componentName]

		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(appComponent, env)

		var variables v1.EnvVarsMap
		monitoring := false
		var resources v1.ResourceRequirements

		// Will later be overriden by default replicas if not set specifically
		var replicas *int

		var horizontalScaling *v1.RadixHorizontalScaling
		var volumeMounts []v1.RadixVolumeMount
		var node v1.RadixNode
		var imageTagName string

		var alwaysPullImageOnDeploy bool
		// Containers run as root unless overridden in config
		runAsNonRoot := false

		image := componentImage.ImagePath

		if environmentSpecificConfig != nil {
			replicas = environmentSpecificConfig.Replicas
			variables = environmentSpecificConfig.Variables
			monitoring = environmentSpecificConfig.Monitoring
			resources = environmentSpecificConfig.Resources
			horizontalScaling = environmentSpecificConfig.HorizontalScaling
			volumeMounts = environmentSpecificConfig.VolumeMounts
			node = environmentSpecificConfig.Node
			imageTagName = environmentSpecificConfig.ImageTagName
			runAsNonRoot = environmentSpecificConfig.RunAsNonRoot
			alwaysPullImageOnDeploy = GetCascadeBoolean(environmentSpecificConfig.AlwaysPullImageOnDeploy, appComponent.AlwaysPullImageOnDeploy, false)
		} else {
			alwaysPullImageOnDeploy = GetCascadeBoolean(nil, appComponent.AlwaysPullImageOnDeploy, false)
		}
		if variables == nil {
			variables = make(v1.EnvVarsMap)
		}
		// Append common environment variables from appComponent.Variables to variables if not available yet
		for variableKey, variableValue := range appComponent.Variables {
			if _, found := variables[variableKey]; !found {
				variables[variableKey] = variableValue
			}
		}

		// Append common resources settings if currently empty
		if reflect.DeepEqual(resources, v1.ResourceRequirements{}) {
			resources = appComponent.Resources
		}

		// For deploy-only images, we will replace the dynamic tag with the tag from the environment
		// config
		if !componentImage.Build && strings.HasSuffix(image, v1.DynamicTagNameInEnvironmentConfig) {
			image = strings.ReplaceAll(image, v1.DynamicTagNameInEnvironmentConfig, imageTagName)
		}

		if len(node.Gpu) <= 0 {
			node.Gpu = appComponent.Node.Gpu
		}
		// Append common environment variables from appComponent.Variables to variables if not available yet
		for variableKey, variableValue := range appComponent.Variables {
			if _, found := variables[variableKey]; !found {
				variables[variableKey] = variableValue
			}
		}

		externalAlias := GetExternalDNSAliasForComponentEnvironment(radixApplication, componentName, env)
		deployComponent := v1.RadixDeployComponent{
			Name:                    componentName,
			RunAsNonRoot:            runAsNonRoot,
			Image:                   image,
			Replicas:                replicas,
			Public:                  false,
			PublicPort:              getPublicPortFromAppComponent(appComponent),
			Ports:                   appComponent.Ports,
			Secrets:                 appComponent.Secrets,
			IngressConfiguration:    appComponent.IngressConfiguration,
			EnvironmentVariables:    variables, // todo: use single EnvVars instead
			DNSAppAlias:             IsDNSAppAlias(env, componentName, dnsAppAlias),
			DNSExternalAlias:        externalAlias,
			Monitoring:              monitoring,
			Resources:               resources,
			HorizontalScaling:       horizontalScaling,
			VolumeMounts:            volumeMounts,
			Node:                    node,
			AlwaysPullImageOnDeploy: alwaysPullImageOnDeploy,
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

func GetCascadeBoolean(first *bool, second *bool, fallback bool) bool {
	if first != nil {
		return *first
	} else if second != nil {
		return *second
	} else {
		return fallback
	}
}
