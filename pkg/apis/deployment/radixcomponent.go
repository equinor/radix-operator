package deployment

import (
	"errors"
	"fmt"

	"dario.cat/mergo"
	commonutils "github.com/equinor/radix-common/utils"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

var (
	booleanPointerTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
)

func GetRadixComponentsForEnv(radixApplication *radixv1.RadixApplication, env string, componentImages pipeline.DeployComponentImages, defaultEnvVars radixv1.EnvVarsMap, preservingDeployComponents []radixv1.RadixDeployComponent) ([]radixv1.RadixDeployComponent, error) {
	dnsAppAlias := radixApplication.Spec.DNSAppAlias
	var deployComponents []radixv1.RadixDeployComponent
	preservingDeployComponentMap := slice.Reduce(preservingDeployComponents, make(map[string]radixv1.RadixDeployComponent), func(acc map[string]radixv1.RadixDeployComponent, component radixv1.RadixDeployComponent) map[string]radixv1.RadixDeployComponent {
		acc[component.GetName()] = component
		return acc
	})
	for _, radixComponent := range radixApplication.Spec.Components {
		environmentSpecificConfig := getEnvironmentSpecificConfigForComponent(radixComponent, env)
		if !radixComponent.GetEnabledForEnvironmentConfig(environmentSpecificConfig) {
			continue
		}
		componentName := radixComponent.Name
		if preservingDeployComponent, ok := preservingDeployComponentMap[componentName]; ok {
			deployComponents = append(deployComponents, preservingDeployComponent)
			continue
		}
		deployComponent := radixv1.RadixDeployComponent{
			Name:                 componentName,
			Public:               false,
			IngressConfiguration: radixComponent.IngressConfiguration,
			Ports:                radixComponent.Ports,
			MonitoringConfig:     radixComponent.MonitoringConfig,
			Secrets:              radixComponent.Secrets,
			DNSAppAlias:          IsDNSAppAlias(env, componentName, dnsAppAlias),
		}
		if environmentSpecificConfig != nil {
			deployComponent.Replicas = environmentSpecificConfig.Replicas
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
		deployComponent.Image, err = getImagePath(componentName, componentImage, radixComponent.ImageTagName, environmentSpecificConfig)
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
		deployComponent.ReadOnlyFileSystem = getRadixCommonComponentReadOnlyFileSystem(&radixComponent, environmentSpecificConfig)
		deployComponent.Monitoring = getRadixCommonComponentMonitoring(&radixComponent, environmentSpecificConfig)
		if deployComponent.HorizontalScaling, err = getRadixCommonComponentHorizontalScaling(&radixComponent, environmentSpecificConfig); err != nil {
			return nil, err
		}
		if deployComponent.VolumeMounts, err = getRadixCommonComponentVolumeMounts(&radixComponent, environmentSpecificConfig); err != nil {
			return nil, err
		}
		deployComponents = append(deployComponents, deployComponent)
	}

	return deployComponents, nil
}

func getRadixCommonComponentReadOnlyFileSystem(radixComponent radixv1.RadixCommonComponent, environmentSpecificConfig radixv1.RadixCommonEnvironmentConfig) *bool {
	if !commonutils.IsNil(environmentSpecificConfig) && environmentSpecificConfig.GetReadOnlyFileSystem() != nil {
		return environmentSpecificConfig.GetReadOnlyFileSystem()
	}
	return radixComponent.GetReadOnlyFileSystem()
}

func getRadixCommonComponentMonitoring(radixComponent radixv1.RadixCommonComponent, environmentSpecificConfig radixv1.RadixCommonEnvironmentConfig) bool {
	if !commonutils.IsNil(environmentSpecificConfig) && environmentSpecificConfig.GetMonitoring() != nil {
		return *environmentSpecificConfig.GetMonitoring()
	}
	monitoring := radixComponent.GetMonitoring()
	return !commonutils.IsNil(monitoring) && *monitoring
}

func getRadixCommonComponentHorizontalScaling(radixComponent radixv1.RadixCommonComponent, environmentSpecificConfig radixv1.RadixCommonEnvironmentConfig) (*radixv1.RadixHorizontalScaling, error) {
	if commonutils.IsNil(environmentSpecificConfig) || environmentSpecificConfig.GetHorizontalScaling() == nil {
		return radixComponent.GetHorizontalScaling().NormalizeConfig(), nil
	}

	environmentHorizontalScaling := environmentSpecificConfig.GetHorizontalScaling()
	if radixComponent.GetHorizontalScaling() == nil {
		return environmentHorizontalScaling.NormalizeConfig(), nil
	}

	finalHorizontalScaling := radixComponent.GetHorizontalScaling().NormalizeConfig()
	if environmentHorizontalScaling.MinReplicas != nil {
		finalHorizontalScaling.MinReplicas = environmentHorizontalScaling.MinReplicas
	}
	if environmentHorizontalScaling.MaxReplicas > 0 && (finalHorizontalScaling.MinReplicas == nil || *finalHorizontalScaling.MinReplicas < environmentHorizontalScaling.MaxReplicas) {
		finalHorizontalScaling.MaxReplicas = environmentHorizontalScaling.MaxReplicas
	}
	if environmentHorizontalScaling.CooldownPeriod != nil {
		finalHorizontalScaling.CooldownPeriod = environmentHorizontalScaling.CooldownPeriod
	}
	if environmentHorizontalScaling.PollingInterval != nil {
		finalHorizontalScaling.PollingInterval = environmentHorizontalScaling.PollingInterval
	}
	// TODO Write tests

	// If original env config has triggers, use that instead of component level triggers. No merging should happen.
	// (We cannot compare normalized config, since it adds default CPU trigger
	if environmentHorizontalScaling.Triggers != nil || environmentHorizontalScaling.RadixHorizontalScalingResources != nil {
		finalHorizontalScaling.Triggers = environmentHorizontalScaling.NormalizeConfig().Triggers
	}
	return finalHorizontalScaling, nil

}

func getRadixCommonComponentVolumeMounts(radixComponent radixv1.RadixCommonComponent, environmentSpecificConfig radixv1.RadixCommonEnvironmentConfig) ([]radixv1.RadixVolumeMount, error) {
	componentVolumeMounts := radixComponent.GetVolumeMounts()
	if commonutils.IsNil(environmentSpecificConfig) || environmentSpecificConfig.GetVolumeMounts() == nil {
		return componentVolumeMounts, nil
	}
	environmentVolumeMounts := environmentSpecificConfig.GetVolumeMounts()
	if componentVolumeMounts == nil {
		return environmentVolumeMounts, nil
	}
	environmentVolumeMountMap := slice.Reduce(environmentVolumeMounts, make(map[string]radixv1.RadixVolumeMount), func(acc map[string]radixv1.RadixVolumeMount, volumeMount radixv1.RadixVolumeMount) map[string]radixv1.RadixVolumeMount {
		environmentVolumeMount := volumeMount
		acc[volumeMount.Name] = environmentVolumeMount
		return acc
	})
	var errs []error
	var finalVolumeMounts []radixv1.RadixVolumeMount
	for _, componentVolumeMount := range componentVolumeMounts {
		finalVolumeMount := componentVolumeMount.DeepCopy()
		volumeMountName := componentVolumeMount.Name
		if envVolumeMount, ok := environmentVolumeMountMap[volumeMountName]; ok {
			if err := mergo.Merge(finalVolumeMount, envVolumeMount, mergo.WithOverride, mergo.WithTransformers(booleanPointerTransformer)); err != nil {
				errs = append(errs, fmt.Errorf("failed to merge component and environment volume-mounts %s: %w", volumeMountName, err))
			}
			delete(environmentVolumeMountMap, volumeMountName)
		}
		finalVolumeMounts = append(finalVolumeMounts, *finalVolumeMount)
	}
	for _, environmentVolumeMount := range environmentVolumeMountMap {
		volumeMount := environmentVolumeMount
		finalVolumeMounts = append(finalVolumeMounts, volumeMount)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return finalVolumeMounts, nil
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

	if err := mergo.Merge(authBase, authEnv, mergo.WithOverride, mergo.WithTransformers(booleanPointerTransformer)); err != nil {
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
