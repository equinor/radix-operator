package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func GetRadixComponentsForEnv(radixApplication *v1.RadixApplication, env string, componentImages map[string]pipeline.ComponentImage) []v1.RadixDeployComponent {
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	var components []v1.RadixDeployComponent

	for _, radixComponent := range radixApplication.Spec.Components {
		componentName := radixComponent.Name
		deployComponent := v1.RadixDeployComponent{
			Name:                 componentName,
			Environment:          env,
			Public:               false,
			IngressConfiguration: radixComponent.IngressConfiguration,
			Ports:                radixComponent.Ports,
			Secrets:              radixComponent.Secrets,
			DNSAppAlias:          IsDNSAppAlias(env, componentName, dnsAppAlias),
			Monitoring:           false,
			RunAsNonRoot:         false,
		}

		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(radixComponent, env)
		if environmentSpecificConfig != nil {
			deployComponent.Replicas = environmentSpecificConfig.Replicas
			deployComponent.Monitoring = environmentSpecificConfig.Monitoring
			deployComponent.HorizontalScaling = environmentSpecificConfig.HorizontalScaling
			deployComponent.VolumeMounts = environmentSpecificConfig.VolumeMounts
			deployComponent.Environment = environmentSpecificConfig.Environment
			deployComponent.RunAsNonRoot = environmentSpecificConfig.RunAsNonRoot
		}

		componentImage := componentImages[componentName]
		deployComponent.Image = getImagePath(&componentImage, environmentSpecificConfig)
		deployComponent.Node = getRadixCommonComponentNode(&radixComponent, environmentSpecificConfig)
		deployComponent.Resources = getRadixCommonComponentResources(&radixComponent, environmentSpecificConfig)
		deployComponent.EnvironmentVariables = getRadixCommonComponentEnvVars(&radixComponent, environmentSpecificConfig)
		deployComponent.AlwaysPullImageOnDeploy = getRadixComponentAlwaysPullImageOnDeployFlag(&radixComponent, environmentSpecificConfig)
		deployComponent.Authentication = getRadixComponentAuthentication(&radixComponent, environmentSpecificConfig)
		deployComponent.DNSExternalAlias = GetExternalDNSAliasForComponentEnvironment(radixApplication, componentName, env)
		deployComponent.SecretRefs = getRadixCommonComponentRadixSecretRefs(&radixComponent, environmentSpecificConfig)
		deployComponent.PublicPort = getRadixComponentPort(&radixComponent)

		components = append(components, deployComponent)
	}

	return components
}

func getRadixComponentAuthentication(radixComponent *v1.RadixComponent, environmentSpecificConfig *v1.RadixEnvironmentConfig) *v1.Authentication {
	var environmentAuthentication *v1.Authentication
	if environmentSpecificConfig != nil {
		environmentAuthentication = environmentSpecificConfig.Authentication
	}
	authentication := GetAuthenticationForComponent(radixComponent.Authentication, environmentAuthentication)
	return authentication
}

func getRadixComponentAlwaysPullImageOnDeployFlag(radixComponent *v1.RadixComponent, environmentSpecificConfig *v1.RadixEnvironmentConfig) bool {
	if environmentSpecificConfig != nil {
		return GetCascadeBoolean(environmentSpecificConfig.AlwaysPullImageOnDeploy, radixComponent.AlwaysPullImageOnDeploy, false)
	}
	return GetCascadeBoolean(nil, radixComponent.AlwaysPullImageOnDeploy, false)
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
		}
	}

	return authentication
}

func GetClientCertificateForComponent(componentCertificate *v1.ClientCertificate, environmentCertificate *v1.ClientCertificate) *v1.ClientCertificate {
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

func getRadixComponentPort(radixComponent *v1.RadixComponent) string {
	if radixComponent.PublicPort == "" && radixComponent.Public {
		return radixComponent.Ports[0].Name
	}
	return radixComponent.PublicPort
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
