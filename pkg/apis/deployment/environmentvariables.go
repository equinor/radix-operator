package deployment

import (
	"context"
	"fmt"
	"sort"
	"strings"

	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

type environmentVariablesSourceDecorator interface {
	getClusterName(ctx context.Context) (string, error)
	getContainerRegistry() (string, error)
	getDnsZone() (string, error)
	getClusterType() (string, error)
	getClusterActiveEgressIps(ctx context.Context) (string, error)
}

type radixApplicationEnvironmentVariablesSourceDecorator struct{}
type radixOperatorEnvironmentVariablesSourceDecorator struct {
	kubeutil *kube.Kube
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getClusterName(_ context.Context) (string, error) {
	return defaults.GetEnvVar(defaults.ClusternameEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getContainerRegistry() (string, error) {
	return defaults.GetEnvVar(defaults.ContainerRegistryEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getDnsZone() (string, error) {
	return defaults.GetEnvVar(defaults.RadixDNSZoneEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getClusterType() (string, error) {
	return defaults.GetEnvVar(defaults.RadixClusterTypeEnvironmentVariable)
}

func (envVarsSource *radixApplicationEnvironmentVariablesSourceDecorator) getClusterActiveEgressIps(_ context.Context) (string, error) {
	return defaults.GetEnvVar(defaults.RadixActiveClusterEgressIpsEnvironmentVariable)
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getClusterName(ctx context.Context) (string, error) {
	clusterName, err := envVarsSource.kubeutil.GetClusterName(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster name from ConfigMap: %v", err)
	}
	return clusterName, nil
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getContainerRegistry() (string, error) {
	containerRegistry, err := defaults.GetEnvVar(defaults.ContainerRegistryEnvironmentVariable)
	if err != nil {
		return "", fmt.Errorf("failed to get container registry from ConfigMap: %v", err)
	}
	return containerRegistry, nil
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getDnsZone() (string, error) {
	return defaults.GetEnvVar(defaults.OperatorDNSZoneEnvironmentVariable)
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getClusterType() (string, error) {
	return defaults.GetEnvVar(defaults.OperatorClusterTypeEnvironmentVariable)
}

func (envVarsSource *radixOperatorEnvironmentVariablesSourceDecorator) getClusterActiveEgressIps(ctx context.Context) (string, error) {
	egressIps, err := envVarsSource.kubeutil.GetClusterActiveEgressIps(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster egress IPs from ConfigMap: %v", err)
	}
	return egressIps, nil
}

// GetEnvironmentVariablesForRadixOperator Provides RADIX_* environment variables for Radix operator.
// It requires service account having access to config map in default namespace.
func GetEnvironmentVariablesForRadixOperator(ctx context.Context, kubeutil *kube.Kube, appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return getEnvironmentVariablesFrom(ctx, kubeutil, appName, &radixOperatorEnvironmentVariablesSourceDecorator{kubeutil: kubeutil}, radixDeployment, deployComponent)
}

// GetEnvironmentVariables Provides environment variables for Radix application.
func GetEnvironmentVariables(ctx context.Context, kubeutil *kube.Kube, appName string, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	return getEnvironmentVariablesFrom(ctx, kubeutil, appName, &radixApplicationEnvironmentVariablesSourceDecorator{}, radixDeployment, deployComponent)
}

func getEnvironmentVariablesFrom(ctx context.Context, kubeutil *kube.Kube, appName string, envVarsSource environmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ([]corev1.EnvVar, error) {
	envVarsConfigMap, _, err := kubeutil.GetOrCreateEnvVarsConfigMapAndMetadataMap(ctx, radixDeployment.GetNamespace(),
		appName, deployComponent.GetName())
	if err != nil {
		return nil, err
	}

	return getEnvironmentVariables(ctx, appName, envVarsSource, radixDeployment, deployComponent, envVarsConfigMap), nil
}

func getEnvironmentVariables(ctx context.Context, appName string, envVarsSource environmentVariablesSourceDecorator, radixDeployment *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent, envVarConfigMap *corev1.ConfigMap) []corev1.EnvVar {
	var (
		namespace          = radixDeployment.Namespace
		currentEnvironment = radixDeployment.Spec.Environment
	)

	var envVars = getEnvVars(envVarConfigMap, deployComponent.GetEnvironmentVariables())

	envVars = appendDefaultEnvVars(ctx, envVars, envVarsSource, currentEnvironment, namespace, appName, deployComponent)

	if !isDeployComponentJobSchedulerDeployment(deployComponent) { // JobScheduler does not need env-vars for secrets and secret-refs
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

func appendDefaultEnvVars(ctx context.Context, envVars []corev1.EnvVar, envVarsSource environmentVariablesSourceDecorator, currentEnvironment, namespace, appName string, deployComponent v1.RadixCommonDeployComponent) []corev1.EnvVar {
	logger := log.Ctx(ctx)
	envVarSet := utils.NewEnvironmentVariablesSet().Init(envVars)
	dnsZone, err := envVarsSource.getDnsZone()
	if err != nil {
		// TODO: Should the error be returned to caller
		logger.Error().Err(err).Msg("Failed to get DNS zone")
		return envVarSet.Items()
	}

	clusterType, err := envVarsSource.getClusterType()
	if err == nil {
		envVarSet.Add(defaults.RadixClusterTypeEnvironmentVariable, clusterType)
	} else {
		// TODO: Should the error be returned to caller
		logger.Debug().Err(err).Msg("Failed to get cluster type")
	}
	containerRegistry, err := envVarsSource.getContainerRegistry()
	if err != nil {
		// TODO: Should the error be returned to caller
		logger.Error().Err(err).Msg("Failed to get container registry")
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ContainerRegistryEnvironmentVariable, containerRegistry)
	envVarSet.Add(defaults.RadixDNSZoneEnvironmentVariable, dnsZone)
	clusterName, err := envVarsSource.getClusterName(ctx)
	if err != nil {
		// TODO: Should the error be returned to caller
		logger.Error().Err(err).Msg("Failed to get cluster name")
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.ClusternameEnvironmentVariable, clusterName)
	envVarSet.Add(defaults.EnvironmentnameEnvironmentVariable, currentEnvironment)
	isPortPublic := deployComponent.GetPublicPort() != "" || deployComponent.IsPublic()
	if isPortPublic {
		canonicalHostName := getHostName(deployComponent.GetName(), namespace, clusterName, dnsZone)
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

	activeClusterEgressIps, err := envVarsSource.getClusterActiveEgressIps(ctx)
	if err != nil {
		// TODO: Should the error be returned to caller
		logger.Error().Err(err).Msg("Failed to get active egress IP addresses")
		return envVarSet.Items()
	}
	envVarSet.Add(defaults.RadixActiveClusterEgressIpsEnvironmentVariable, activeClusterEgressIps)

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
