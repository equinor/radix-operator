package deployment

import (
	"dario.cat/mergo"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

var (
	authTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
)

func GetRadixComponentsForEnv(radixApplication *radixv1.RadixApplication, env string, componentImages pipeline.DeployComponentImages, defaultEnvVars radixv1.EnvVarsMap) ([]radixv1.RadixDeployComponent, error) {
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	var components []radixv1.RadixDeployComponent

	for _, radixComponent := range radixApplication.Spec.Components {
		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(radixComponent, env)
		if !radixComponent.GetEnabledForEnv(environmentSpecificConfig) {
			continue
		}
		componentName := radixComponent.Name
		deployComponent := radixv1.RadixDeployComponent{
			Name:                 componentName,
			Public:               false,
			IngressConfiguration: radixComponent.IngressConfiguration,
			Ports:                radixComponent.Ports,
			MonitoringConfig:     radixComponent.MonitoringConfig,
			Secrets:              radixComponent.Secrets,
			DNSAppAlias:          IsDNSAppAlias(env, componentName, dnsAppAlias),
			Monitoring:           false,
		}
		if environmentSpecificConfig != nil {
			deployComponent.Replicas = environmentSpecificConfig.Replicas
			deployComponent.Monitoring = environmentSpecificConfig.Monitoring
			deployComponent.HorizontalScaling = environmentSpecificConfig.HorizontalScaling
			deployComponent.VolumeMounts = environmentSpecificConfig.VolumeMounts
		}

		auth, err := getRadixComponentAuthentication(&radixComponent, environmentSpecificConfig)
		if err != nil {
			return nil, err
		}

		identity, err := getRadixCommonComponentIdentity(&radixComponent, environmentSpecificConfig)
		if err != nil {
			return nil, err
		}

		componentImage := componentImages[componentName]
		deployComponent.Image, err = getImagePath(componentName, componentImage, environmentSpecificConfig)
		if err != nil {
			return nil, err
		}
		deployComponent.Node = getRadixCommonComponentNode(&radixComponent, environmentSpecificConfig)
		deployComponent.Resources = getRadixCommonComponentResources(&radixComponent, environmentSpecificConfig)
		deployComponent.EnvironmentVariables = getRadixCommonComponentEnvVars(&radixComponent, environmentSpecificConfig, defaultEnvVars)
		deployComponent.AlwaysPullImageOnDeploy = getRadixComponentAlwaysPullImageOnDeployFlag(&radixComponent, environmentSpecificConfig)
		deployComponent.ExternalDNS = getExternalDNSAliasForComponentEnvironment(radixApplication, componentName, env)
		deployComponent.SecretRefs = getRadixCommonComponentRadixSecretRefs(&radixComponent, environmentSpecificConfig)
		deployComponent.PublicPort = getRadixComponentPort(&radixComponent)
		deployComponent.Authentication = auth
		deployComponent.Identity = identity

		components = append(components, deployComponent)
	}

	return components, nil
}

func getRadixComponentAlwaysPullImageOnDeployFlag(radixComponent *radixv1.RadixComponent, environmentSpecificConfig *radixv1.RadixEnvironmentConfig) bool {
	if environmentSpecificConfig != nil {
		return coalesceBool(environmentSpecificConfig.AlwaysPullImageOnDeploy, radixComponent.AlwaysPullImageOnDeploy, false)
	}
	return coalesceBool(nil, radixComponent.AlwaysPullImageOnDeploy, false)
}

func getRadixComponentAuthentication(radixComponent *radixv1.RadixComponent, environmentSpecificConfig *radixv1.RadixEnvironmentConfig) (*radixv1.Authentication, error) {
	var environmentAuthentication *radixv1.Authentication
	if environmentSpecificConfig != nil {
		environmentAuthentication = environmentSpecificConfig.Authentication
	}
	return GetAuthenticationForComponent(radixComponent.Authentication, environmentAuthentication)
}

func GetAuthenticationForComponent(componentAuthentication *radixv1.Authentication, environmentAuthentication *radixv1.Authentication) (auth *radixv1.Authentication, err error) {
	// mergo uses the reflect package, and reflect use panic() when errors are detected
	// We handle panics to prevent process termination even if the RD will be re-queued forever (until a new RD is built)
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()

	if componentAuthentication == nil && environmentAuthentication == nil {
		return nil, nil
	}

	authBase := &radixv1.Authentication{}
	if componentAuthentication != nil {
		authBase = componentAuthentication.DeepCopy()
	}
	authEnv := &radixv1.Authentication{}
	if environmentAuthentication != nil {
		authEnv = environmentAuthentication.DeepCopy()
	}

	if err := mergo.Merge(authBase, authEnv, mergo.WithOverride, mergo.WithTransformers(authTransformer)); err != nil {
		return nil, err
	}
	return authBase, nil
}

// IsDNSAppAlias Checks if environment and component represents the DNS app alias
func IsDNSAppAlias(env, componentName string, dnsAppAlias radixv1.AppAlias) bool {
	return env == dnsAppAlias.Environment && componentName == dnsAppAlias.Component
}

func getEnvironmentSpecificConfigForComponent(component radixv1.RadixComponent, env string) *radixv1.RadixEnvironmentConfig {
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

func getRadixComponentPort(radixComponent *radixv1.RadixComponent) string {
	if radixComponent.PublicPort == "" && radixComponent.Public {
		return radixComponent.Ports[0].Name
	}
	return radixComponent.PublicPort
}

func getExternalDNSAliasForComponentEnvironment(radixApplication *radixv1.RadixApplication, component, env string) []radixv1.RadixDeployExternalDNS {
	dnsExternalAlias := make([]radixv1.RadixDeployExternalDNS, 0)

	for _, externalAlias := range radixApplication.Spec.DNSExternalAlias {
		if externalAlias.Component == component && externalAlias.Environment == env {
			dnsExternalAlias = append(dnsExternalAlias, radixv1.RadixDeployExternalDNS{FQDN: externalAlias.Alias, UseCertificateAutomation: externalAlias.UseCertificateAutomation})
		}
	}

	return dnsExternalAlias
}

func coalesceBool(first *bool, second *bool, fallback bool) bool {
	if first != nil {
		return *first
	} else if second != nil {
		return *second
	} else {
		return fallback
	}
}
