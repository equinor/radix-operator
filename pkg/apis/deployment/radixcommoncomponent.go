package deployment

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/imdario/mergo"
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

func getImagePath(componentImage *pipeline.ComponentImage, environmentSpecificConfig v1.RadixCommonEnvironmentConfig) string {
	var imageTagName string
	if !commonUtils.IsNil(environmentSpecificConfig) {
		imageTagName = environmentSpecificConfig.GetImageTagName()
	}

	image := componentImage.ImagePath
	// For deploy-only images, we will replace the dynamic tag with the tag from the environment
	// config
	if !componentImage.Build && strings.HasSuffix(image, v1.DynamicTagNameInEnvironmentConfig) {
		image = strings.ReplaceAll(image, v1.DynamicTagNameInEnvironmentConfig, imageTagName)
	}
	return image
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
		if !existsEnvAzureKeyVault { //Azure key vault, which configuration exists only in component, but not in environment
			keyVaultItems := getAzureKeyVaultItemsWithEnvVarsNotExistingInEnvSecretRefs(&commonAzureKeyVault, envSecretRefsExistingEnvVarsMap)
			envAzureKeyVaultsMap[commonAzureKeyVault.Name] = v1.RadixAzureKeyVault{
				Name:  commonAzureKeyVault.Name,
				Path:  commonAzureKeyVault.Path,
				Items: keyVaultItems,
			}
			continue
		}

		composedAzureKeyVault := v1.RadixAzureKeyVault{
			Name:  envAzureKeyVault.Name,
			Path:  commonAzureKeyVault.Path,
			Items: append(envAzureKeyVault.Items, getAzureKeyVaultItemsWithEnvVarsNotExistingInEnvSecretRefs(&commonAzureKeyVault, envSecretRefsExistingEnvVarsMap)...),
		}
		if envAzureKeyVault.Path != nil && len(*envAzureKeyVault.Path) > 0 { //override common path by env-path, if specified in env, or set non-empty env path
			composedAzureKeyVault.Path = envAzureKeyVault.Path
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

	return identity, nil
}
