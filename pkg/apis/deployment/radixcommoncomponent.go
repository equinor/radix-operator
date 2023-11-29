package deployment

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"dario.cat/mergo"
	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func updateComponentNode(component v1.RadixCommonComponent, node *v1.RadixNode) {
	if len(node.Gpu) <= 0 {
		node.Gpu = component.GetNode().Gpu
	}

	nodeGpuCount := node.GpuCount
	if len(nodeGpuCount) <= 0 {
		node.GpuCount = component.GetNode().GpuCount
		return
	}
	if gpuCount, err := strconv.Atoi(nodeGpuCount); err != nil || gpuCount <= 0 {
		log.Error(fmt.Sprintf("invalid environment node GPU count: %s in component %s", nodeGpuCount, component.GetName()))
		node.GpuCount = component.GetNode().GpuCount
	}
}

func getRadixCommonComponentEnvVars(component v1.RadixCommonComponent, environmentSpecificConfig v1.RadixCommonEnvironmentConfig, defaultEnvVars v1.EnvVarsMap) v1.EnvVarsMap {
	if !component.GetEnabledForEnv(environmentSpecificConfig) {
		return make(v1.EnvVarsMap)
	}
	var variables v1.EnvVarsMap
	if !commonUtils.IsNil(environmentSpecificConfig) {
		variables = environmentSpecificConfig.GetVariables()
	}
	if variables == nil {
		variables = make(v1.EnvVarsMap)
	}

	// Append common environment variables from radixComponent.Variables to variables if not available yet
	for variableKey, variableValue := range component.GetVariables() {
		if _, found := variables[variableKey]; !found {
			variables[variableKey] = variableValue
		}
	}

	// Append system default environment variables
	for key, value := range defaultEnvVars {
		variables[key] = value
	}

	return variables
}

func getRadixCommonComponentResources(component v1.RadixCommonComponent, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) v1.ResourceRequirements {
	var resources v1.ResourceRequirements
	if !commonUtils.IsNil(environmentSpecificConfig) {
		resources = environmentSpecificConfig.GetResources()
	}
	if reflect.DeepEqual(resources, v1.ResourceRequirements{}) {
		resources = component.GetResources()
	}
	return resources
}

func getRadixCommonComponentNode(radixComponent v1.RadixCommonComponent, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) v1.RadixNode {
	var node v1.RadixNode
	if !commonUtils.IsNil(environmentSpecificConfig) {
		node = environmentSpecificConfig.GetNode()
	}
	updateComponentNode(radixComponent, &node)
	return node
}

func getImagePath(componentName string, componentImage pipeline.DeployComponentImage, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) (string, error) {
	image := componentImage.ImagePath
	if componentImage.Build {
		return image, nil
	}

	imageTagName := getImageTagName(componentImage, environmentSpecificConfig)
	if strings.HasSuffix(image, v1.DynamicTagNameInEnvironmentConfig) {
		if len(imageTagName) == 0 {
			return "", errorMissingExpectedDynamicImageTagName(componentName)
		}
		// For deploy-only images, we will replace the dynamic tag with the tag from the environment config
		return strings.ReplaceAll(image, v1.DynamicTagNameInEnvironmentConfig, imageTagName), nil
	}
	return image, nil
}

func errorMissingExpectedDynamicImageTagName(componentName string) error {
	return fmt.Errorf(fmt.Sprintf("component %s is missing an expected dynamic imageTagName for its image", componentName))
}

func getImageTagName(componentImage pipeline.DeployComponentImage, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) string {
	if componentImage.ImageTagName != "" {
		return componentImage.ImageTagName // provided via radix-api build request
	}
	if !commonUtils.IsNil(environmentSpecificConfig) {
		return environmentSpecificConfig.GetImageTagName()
	}
	return ""
}

func getRadixCommonComponentRadixSecretRefs(component v1.RadixCommonComponent, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) v1.RadixSecretRefs {
	return v1.RadixSecretRefs{
		AzureKeyVaults: getRadixCommonComponentAzureKeyVaultSecretRefs(component, environmentSpecificConfig),
	}
}

func getRadixCommonComponentAzureKeyVaultSecretRefs(radixComponent v1.RadixCommonComponent, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) []v1.RadixAzureKeyVault {
	if len(radixComponent.GetSecretRefs().AzureKeyVaults) == 0 {
		if !commonUtils.IsNil(environmentSpecificConfig) {
			return environmentSpecificConfig.GetSecretRefs().AzureKeyVaults
		}
		return nil
	}

	envAzureKeyVaultsMap := make(map[string]v1.RadixAzureKeyVault)
	envSecretRefsExistingEnvVarsMap := make(map[string]bool)

	if !commonUtils.IsNil(environmentSpecificConfig) {
		for _, envAzureKeyVault := range environmentSpecificConfig.GetSecretRefs().AzureKeyVaults {
			envAzureKeyVaultsMap[envAzureKeyVault.Name] = envAzureKeyVault
			for _, envKeyVaultItem := range envAzureKeyVault.Items {
				envSecretRefsExistingEnvVarsMap[envKeyVaultItem.EnvVar] = true
			}
		}
	}

	for _, commonAzureKeyVault := range radixComponent.GetSecretRefs().AzureKeyVaults {
		if len(commonAzureKeyVault.Items) == 0 {
			continue
		}
		envAzureKeyVault, existsEnvAzureKeyVault := envAzureKeyVaultsMap[commonAzureKeyVault.Name]
		if !existsEnvAzureKeyVault { // Azure key vault, which configuration exists only in component, but not in environment
			keyVaultItems := getAzureKeyVaultItemsWithEnvVarsNotExistingInEnvSecretRefs(&commonAzureKeyVault, envSecretRefsExistingEnvVarsMap)
			envAzureKeyVaultsMap[commonAzureKeyVault.Name] = v1.RadixAzureKeyVault{
				Name:             commonAzureKeyVault.Name,
				Path:             commonAzureKeyVault.Path,
				UseAzureIdentity: commonAzureKeyVault.UseAzureIdentity,
				Items:            keyVaultItems,
			}
			continue
		}

		composedAzureKeyVault := v1.RadixAzureKeyVault{
			Name:             envAzureKeyVault.Name,
			Path:             commonAzureKeyVault.Path,
			UseAzureIdentity: commonAzureKeyVault.UseAzureIdentity,
			Items:            append(envAzureKeyVault.Items, getAzureKeyVaultItemsWithEnvVarsNotExistingInEnvSecretRefs(&commonAzureKeyVault, envSecretRefsExistingEnvVarsMap)...),
		}
		if envAzureKeyVault.Path != nil && len(*envAzureKeyVault.Path) > 0 { // override common path by env-path, if specified in env, or set non-empty env path
			composedAzureKeyVault.Path = envAzureKeyVault.Path
		}
		if envAzureKeyVault.UseAzureIdentity != nil { // override common useAzureIdentity by env, if specified in env
			composedAzureKeyVault.UseAzureIdentity = envAzureKeyVault.UseAzureIdentity
		}

		envAzureKeyVaultsMap[commonAzureKeyVault.Name] = composedAzureKeyVault
	}

	var azureKeyVaults []v1.RadixAzureKeyVault
	for _, azureKeyVault := range envAzureKeyVaultsMap {
		if len(azureKeyVault.Items) > 0 {
			azureKeyVaults = append(azureKeyVaults, azureKeyVault)
		}
	}
	return azureKeyVaults
}

func getAzureKeyVaultItemsWithEnvVarsNotExistingInEnvSecretRefs(commonAzureKeyVault *v1.RadixAzureKeyVault, envSecretRefsExistingEnvVarsMap map[string]bool) []v1.RadixAzureKeyVaultItem {
	var keyVaultItems []v1.RadixAzureKeyVaultItem
	for _, commonKeyVaultItem := range commonAzureKeyVault.Items {
		if _, existsInEnvSecretRefs := envSecretRefsExistingEnvVarsMap[commonKeyVaultItem.EnvVar]; !existsInEnvSecretRefs {
			keyVaultItems = append(keyVaultItems, commonKeyVaultItem)
		}
	}
	return keyVaultItems
}

func getRadixCommonComponentIdentity(radixComponent v1.RadixCommonComponent, environmentConfig v1.RadixCommonEnvironmentConfig) (identity *v1.Identity, err error) {
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

	identity = &v1.Identity{}

	if !commonUtils.IsNil(radixComponent) {
		if componentIdentity := radixComponent.GetIdentity(); componentIdentity != nil {
			componentIdentity.DeepCopyInto(identity)
		}
	}

	if !commonUtils.IsNil(environmentConfig) {
		if environmentIdentity := environmentConfig.GetIdentity(); environmentIdentity != nil {
			if err := mergo.Merge(identity, environmentIdentity, mergo.WithOverride); err != nil {
				return nil, err
			}
		}
	}

	if reflect.DeepEqual(identity, &v1.Identity{}) {
		return nil, nil
	}

	if identity != nil && identity.Azure != nil {
		id, err := uuid.Parse(identity.Azure.ClientId)
		if err != nil {
			return nil, fmt.Errorf("failed to parse identity.azure.clientId for component %s: %w", radixComponent.GetName(), err)
		}
		identity.Azure.ClientId = id.String()
	}

	return identity, nil
}

func getRadixJobComponentNotification(radixComponent *v1.RadixJobComponent, environmentConfig *v1.RadixJobComponentEnvironmentConfig) (notifications *v1.Notifications, err error) {
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

	notifications = &v1.Notifications{}

	if !commonUtils.IsNil(radixComponent) {
		if componentNotifications := radixComponent.GetNotifications(); componentNotifications != nil {
			componentNotifications.DeepCopyInto(notifications)
		}
	}

	if !commonUtils.IsNil(environmentConfig) {
		if environmentNotifications := environmentConfig.GetNotifications(); environmentNotifications != nil {
			if err := mergo.Merge(notifications, environmentNotifications, mergo.WithOverride); err != nil {
				return nil, err
			}
		}
	}

	if reflect.DeepEqual(notifications, &v1.Notifications{}) {
		return nil, nil
	}

	return notifications, nil
}
