package deployment

import (
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/imdario/mergo"
)

func GetRadixComponentsForEnv(radixApplication *v1.RadixApplication, env string, componentImages map[string]pipeline.ComponentImage) []v1.RadixDeployComponent {
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	components := []v1.RadixDeployComponent{}

	for _, appComponent := range radixApplication.Spec.Components {
		componentName := appComponent.Name
		componentImage := componentImages[componentName]

		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(appComponent, env)

		// Will later be overriden by default replicas if not set specifically
		var replicas *int
		var variables v1.EnvVarsMap
		monitoring := false
		var resources v1.ResourceRequirements
		var horizontalScaling *v1.RadixHorizontalScaling
		var volumeMounts []v1.RadixVolumeMount
		var node v1.RadixNode
		var imageTagName string
		var alwaysPullImageOnDeploy bool
		var environmentAuthentication *v1.Authentication

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
			environmentAuthentication = environmentSpecificConfig.Authentication
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

		updateComponentNode(&appComponent, &node)

		// Append common environment variables from appComponent.Variables to variables if not available yet
		for variableKey, variableValue := range appComponent.Variables {
			if _, found := variables[variableKey]; !found {
				variables[variableKey] = variableValue
			}
		}

		externalAlias := GetExternalDNSAliasForComponentEnvironment(radixApplication, componentName, env)
		authentication := GetAuthenticationForComponent(appComponent.Authentication, environmentAuthentication)

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
			Authentication:          authentication,
		}

		components = append(components, deployComponent)
	}

	return components
}

func GetAuthenticationForComponent(componentAuthentication *v1.Authentication, environmentAuthentication *v1.Authentication) *v1.Authentication {
	var authentication *v1.Authentication

	if componentAuthentication == nil && environmentAuthentication == nil {
		authentication = nil
	} else if componentAuthentication == nil {
		authentication = environmentAuthentication.DeepCopy()
	} else if environmentAuthentication == nil {
		authentication = componentAuthentication.DeepCopy()
	} else {
		authentication = &v1.Authentication{
			ClientCertificate: GetClientCertificateForComponent(componentAuthentication.ClientCertificate, environmentAuthentication.ClientCertificate),
			OAuth2:            GetOAuth2ForComponent(componentAuthentication.OAuth2, environmentAuthentication.OAuth2),
		}
	}

	return authentication
}

func GetOAuth2ForComponent(componentOAuth, environmentOAuth *v1.OAuth2) *v1.OAuth2 {

	switch {
	case componentOAuth == nil && environmentOAuth == nil:
		return nil
	case componentOAuth == nil:
		return environmentOAuth.DeepCopy()
	case environmentOAuth == nil:
		return componentOAuth.DeepCopy()
	}

	dstOAuth := componentOAuth.DeepCopy()
	srcOAuth := environmentOAuth.DeepCopy()
	mergo.Merge(dstOAuth, srcOAuth)
	return dstOAuth
}

func GetClientCertificateForComponent(componentCertificate, environmentCertificate *v1.ClientCertificate) *v1.ClientCertificate {
	var certificate *v1.ClientCertificate
	if componentCertificate == nil && environmentCertificate == nil {
		certificate = nil
	} else if componentCertificate == nil {
		certificate = environmentCertificate.DeepCopy()
	} else if environmentCertificate == nil {
		certificate = componentCertificate.DeepCopy()
	} else {
		certificate = componentCertificate.DeepCopy()
		envCert := environmentCertificate.DeepCopy()
		// mergo.Merge(certificate, envCert)
		if envCert.PassCertificateToUpstream != nil {
			certificate.PassCertificateToUpstream = envCert.PassCertificateToUpstream
		}

		if envCert.Verification != nil {
			certificate.Verification = envCert.Verification
		}
	}

	return certificate
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
