package deployment

import (
	"context"
	"fmt"
	"sort"
	"strings"

	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	internal "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

// GetEnvironmentVariablesForRadixOperator Provides RADIX_* environment variables for Radix operator.
// It requires service account having access to config map in default namespace.
func GetEnvironmentVariablesForRadixOperator(ctx context.Context, kubeutil *kube.Kube, cfg *config.Config, appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	envVarsConfigMap, _, err := kubeutil.GetOrCreateEnvVarsConfigMapAndMetadataMap(ctx, radixDeployment.GetNamespace(),
		appName, deployComponent.GetName())
	if err != nil {
		return nil, err
	}

	return getEnvironmentVariables(ctx, appName, cfg, radixDeployment, deployComponent, envVarsConfigMap), nil
}

func getEnvironmentVariables(ctx context.Context, appName string, cfg *config.Config, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent, envVarConfigMap *corev1.ConfigMap) []corev1.EnvVar {
	var (
		namespace          = radixDeployment.Namespace
		currentEnvironment = radixDeployment.Spec.Environment
	)

	var envVars = getEnvVars(envVarConfigMap, deployComponent.GetEnvironmentVariables())

	envVars = appendDefaultEnvVars(ctx, envVars, cfg, currentEnvironment, namespace, appName, deployComponent)

	if !internal.IsDeployComponentJobSchedulerDeployment(deployComponent) { // JobScheduler does not need env-vars for secrets and secret-refs
		envVars = append(envVars, utils.GetEnvVarsFromSecrets(deployComponent.GetName(), deployComponent.GetSecrets())...)
		envVars = append(envVars, utils.GetEnvVarsFromAzureKeyVaultSecretRefs(radixDeployment.GetName(), deployComponent.GetName(), deployComponent.GetSecretRefs())...)
	}

	// Sorting envVars to prevent unneccessary restart of deployment due to change in the order of envvars
	// range over maps are not guaranteed to be the same from one iteration to the next. https://go.dev/ref/spec#For_statements
	sort.Slice(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	return envVars
}

func getEnvVars(envVarConfigMap *corev1.ConfigMap, deployComponentEnvVars v1.EnvVarsMap) []corev1.EnvVar {
	envVarConfigMapName := envVarConfigMap.GetName()
	// map is not sorted, which lead to random order of env variable in deployment
	// during stop/start/restart of a single component this lead to restart of other several components
	envVarNames := getMapKeysSorted(envVarConfigMap.Data)
	var resultEnvVars []corev1.EnvVar
	usedConfigMapEnvVarNames := map[string]bool{}
	for _, envVarName := range envVarNames {
		if utils.IsRadixEnvVar(envVarName) {
			continue
		}
		resultEnvVars = append(resultEnvVars, createEnvVarWithConfigMapRef(envVarConfigMapName, envVarName))
		usedConfigMapEnvVarNames[envVarName] = true
	}

	// add env-vars, not existing in config-map
	for envVarName, envVarValue := range deployComponentEnvVars {
		if _, ok := usedConfigMapEnvVarNames[envVarName]; !ok {
			resultEnvVars = append(resultEnvVars, corev1.EnvVar{
				Name:  envVarName,
				Value: envVarValue,
			})
		}
	}

	return resultEnvVars
}

func removeFromConfigMapEnvVarsNotExistingInRadixconfig(envVarsMap v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap) {
	envVarCmRefs := radixmaps.GetKeysFromStringMap(envVarConfigMap.Data)
	for _, envVarName := range envVarCmRefs {
		if _, ok := envVarsMap[envVarName]; !ok {
			delete(envVarConfigMap.Data, envVarName)
		}
	}
}

func removeFromConfigMapEnvVarsMetadataNotExistingInEnvVarsConfigMap(envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]kube.EnvVarMetadata) {
	for envVarName := range envVarMetadataMap {
		if _, ok := envVarConfigMap.Data[envVarName]; !ok {
			delete(envVarMetadataMap, envVarName)
		}
	}
}

func createEnvVarWithConfigMapRef(envVarConfigMapName, envVarName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: envVarConfigMapName,
				},
				Key: envVarName,
			},
		},
	}
}

func appendDefaultEnvVars(ctx context.Context, envVars []corev1.EnvVar, cfg *config.Config, currentEnvironment, namespace, appName string, deployComponent v1.RadixCommonDeployComponent) []corev1.EnvVar {
	envVarSet := utils.NewEnvironmentVariablesSet().Init(envVars)

	envVarSet.Add(defaults.RadixClusterTypeEnvironmentVariable, cfg.ClusterType)
	envVarSet.Add(defaults.ContainerRegistryEnvironmentVariable, cfg.ContainerRegistryName)
	envVarSet.Add(defaults.RadixDNSZoneEnvironmentVariable, cfg.DNSZone)
	envVarSet.Add(defaults.ClusternameEnvironmentVariable, cfg.ClusterName)
	envVarSet.Add(defaults.EnvironmentnameEnvironmentVariable, currentEnvironment)
	isPortPublic := deployComponent.GetPublicPort() != "" || deployComponent.IsPublic()
	if isPortPublic {
		canonicalHostName := getHostName(deployComponent.GetName(), namespace, cfg.ClusterName)
		publicHostName := getActiveClusterHostName(deployComponent.GetName(), namespace)
		envVarSet.Add(defaults.PublicEndpointEnvironmentVariable, publicHostName)
		envVarSet.Add(defaults.CanonicalEndpointEnvironmentVariable, canonicalHostName)
	}
	envVarSet.Add(defaults.RadixAppEnvironmentVariable, appName)
	envVarSet.Add(defaults.RadixComponentEnvironmentVariable, deployComponent.GetName())
	ports := deployComponent.GetPorts()
	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		envVarSet.Add(defaults.RadixPortsEnvironmentVariable, portNumbers)
		envVarSet.Add(defaults.RadixPortNamesEnvironmentVariable, portNames)
	}

	return envVarSet.Items()
}

func getPortNumbersAndNamesString(ports []v1.ComponentPort) (string, string) {
	portNumbers := "("
	portNames := "("
	portsSize := len(ports)
	for i, portObj := range ports {
		if i < portsSize-1 {
			portNumbers += fmt.Sprint(portObj.Port) + " "
			portNames += fmt.Sprint(portObj.Name) + " "
		} else {
			portNumbers += fmt.Sprint(portObj.Port) + ")"
			portNames += fmt.Sprint(portObj.Name) + ")"
		}
	}
	return portNumbers, portNames
}

func (deploy *Deployment) createOrUpdateEnvironmentVariableConfigMaps(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) error {
	currentEnvVarsConfigMap, envVarsMetadataConfigMap,
		err := deploy.kubeutil.GetOrCreateEnvVarsConfigMapAndMetadataMap(ctx, deploy.radixDeployment.GetNamespace(),
		deploy.radixDeployment.Spec.AppName, deployComponent.GetName())
	if err != nil {
		return err
	}
	desiredEnvVarsConfigMap := currentEnvVarsConfigMap.DeepCopy()
	envVarsMetadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(ctx, envVarsMetadataConfigMap)
	if err != nil {
		return err
	}

	buildEnvVarsFromRadixConfig(ctx, deployComponent.GetEnvironmentVariables(), desiredEnvVarsConfigMap, envVarsMetadataMap)

	err = deploy.kubeutil.ApplyConfigMap(ctx, deploy.radixDeployment.Namespace, currentEnvVarsConfigMap, desiredEnvVarsConfigMap)
	if err != nil {
		return err
	}
	return deploy.kubeutil.ApplyEnvVarsMetadataConfigMap(ctx, deploy.radixDeployment.Namespace, envVarsMetadataConfigMap, envVarsMetadataMap)
}

func buildEnvVarsFromRadixConfig(ctx context.Context, radixConfigEnvVars v1.EnvVarsMap, envVarConfigMap *corev1.ConfigMap, envVarMetadataMap map[string]kube.EnvVarMetadata) {
	if radixConfigEnvVars == nil {
		log.Ctx(ctx).Debug().Msg("No environment variables are set for this RadixDeployment in radixconfig")
		return
	}

	for envVarName, radixConfigEnvVarValue := range radixConfigEnvVars {
		if utils.IsRadixEnvVar(envVarName) {
			continue
		}
		envVarConfigMapValue, foundValueInEnvVarConfigMap := envVarConfigMap.Data[envVarName]
		envVarMetadata, foundEnvVarMetadata := envVarMetadataMap[envVarName]

		if !foundValueInEnvVarConfigMap || !foundEnvVarMetadata { // no such env-var, created or changed in Radix console
			envVarConfigMap.Data[envVarName] = radixConfigEnvVarValue // use env-var from radixconfig
			if foundEnvVarMetadata {                                  // exists metadata without config-map env-var
				delete(envVarMetadataMap, envVarName) // remove this orphaned metadata
			}
			continue
		}

		if !foundEnvVarMetadata { // no metadata to update
			continue
		}

		if strings.EqualFold(envVarConfigMapValue, envVarMetadata.RadixConfigValue) { // config-map env-var is the same as in metadata
			delete(envVarMetadataMap, envVarName)                     // remove metadata (it is not necessary anymore)
			envVarConfigMap.Data[envVarName] = radixConfigEnvVarValue // use env-var from radixconfig
			continue
		}

		// save radixconfig env-var value to metadata
		envVarMetadata.RadixConfigValue = radixConfigEnvVarValue
		envVarMetadataMap[envVarName] = envVarMetadata
		log.Ctx(ctx).Debug().Msgf("RadixConfig environment variable %s has been set or changed in Radix console", envVarName)
	}
	removeFromConfigMapEnvVarsNotExistingInRadixconfig(radixConfigEnvVars, envVarConfigMap)
	removeFromConfigMapEnvVarsMetadataNotExistingInEnvVarsConfigMap(envVarConfigMap, envVarMetadataMap)
}

func getMapKeysSorted(stringMap map[string]string) []string {
	keys := radixmaps.GetKeysFromStringMap(stringMap)
	sort.Strings(keys)
	return keys
}
