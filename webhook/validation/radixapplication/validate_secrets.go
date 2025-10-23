package radixapplication

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func secretValidator(ctx context.Context, app *radixv1.RadixApplication) (string, error) {
	for _, component := range app.Spec.Components {
		if err := validateRadixComponentSecrets(&component, app); err != nil {
			return "", err
		}
	}
	for _, job := range app.Spec.Jobs {
		if err := validateRadixComponentSecrets(&job, app); err != nil {
			return "", err
		}
	}
	return "", nil
}

func validateRadixComponentSecrets(component radixv1.RadixCommonComponent, app *radixv1.RadixApplication) error {
	envsEnvVarsMap := make(map[string]map[string]bool)

	for _, env := range app.Spec.Environments {
		var envEnvVars radixv1.EnvVarsMap
		if envConfig := component.GetEnvironmentConfigByName(env.Name); envConfig != radixv1.RadixCommonEnvironmentConfig(nil) {
			envEnvVars = envConfig.GetVariables()
		}
		envsEnvVarsMap[env.Name] = getEnvVarNameMap(component.GetVariables(), envEnvVars)
	}

	if err := validateConflictingEnvironmentAndSecretNames(component.GetName(), component.GetSecrets(), envsEnvVarsMap); err != nil {
		return err
	}

	for _, env := range component.GetEnvironmentConfig() {
		envsEnvVarsWithSecretsMap, ok := envsEnvVarsMap[env.GetEnvironment()]
		if !ok {
			continue
		}
		for _, secret := range component.GetSecrets() {
			envsEnvVarsWithSecretsMap[secret] = true
		}
		envsEnvVarsMap[env.GetEnvironment()] = envsEnvVarsWithSecretsMap
	}

	if err := validateRadixComponentSecretRefs(component); err != nil {
		return err
	}

	return validateConflictingEnvironmentAndSecretRefsNames(component, envsEnvVarsMap)
}

func validateConflictingEnvironmentAndSecretNames(componentName string, secrets []string, envsEnvVarMap map[string]map[string]bool) error {
	for _, secret := range secrets {
		for _, envVarMap := range envsEnvVarMap {
			if _, contains := envVarMap[secret]; contains {
				return fmt.Errorf("component %s, secret %s: %w", componentName, secret, ErrSecretNameConflictsWithVariable)
			}
		}
	}
	return nil
}

func validateRadixComponentSecretRefs(radixComponent radixv1.RadixCommonComponent) error {
	err := validateSecretRefs(radixComponent, radixComponent.GetSecretRefs())
	if err != nil {
		return err
	}
	for _, envConfig := range radixComponent.GetEnvironmentConfig() {
		err := validateSecretRefs(radixComponent, envConfig.GetSecretRefs())
		if err != nil {
			return err
		}
	}
	return validateSecretRefsPath(radixComponent)
}

func validateSecretRefs(commonComponent radixv1.RadixCommonComponent, secretRefs radixv1.RadixSecretRefs) error {
	existingVariableName := make(map[string]bool)
	existingAlias := make(map[string]bool)
	existingAzureKeyVaultPath := make(map[string]bool)
	for _, azureKeyVault := range secretRefs.AzureKeyVaults {
		path := azureKeyVault.Path
		if path != nil && len(*path) > 0 {
			if _, exists := existingAzureKeyVaultPath[*path]; exists {
				return fmt.Errorf("azure Key vault %s, component %s, path %s: %w", azureKeyVault.Name, commonComponent.GetName(), *path, ErrDuplicatePathForAzureKeyVault)
			}
			existingAzureKeyVaultPath[*path] = true
		}
		useAzureIdentity := azureKeyVault.UseAzureIdentity
		if useAzureIdentity != nil && *useAzureIdentity {
			if !azureIdentityIsSet(commonComponent) {
				return fmt.Errorf("azure Key vault %s, component %s: %w", azureKeyVault.Name, commonComponent.GetName(), ErrMissingAzureIdentityForAzureKeyVault)
			}
		}
		for _, keyVaultItem := range azureKeyVault.Items {
			if len(keyVaultItem.EnvVar) > 0 {
				if _, exists := existingVariableName[keyVaultItem.EnvVar]; exists {
					return fmt.Errorf("environment variable %s: %w", keyVaultItem.EnvVar, ErrDuplicateEnvVarName)
				}
				existingVariableName[keyVaultItem.EnvVar] = true
			}
			if keyVaultItem.Alias != nil && len(*keyVaultItem.Alias) > 0 {
				if _, exists := existingAlias[*keyVaultItem.Alias]; exists {
					return fmt.Errorf("alias %s: %w", *keyVaultItem.Alias, ErrDuplicateAlias)
				}
				existingAlias[*keyVaultItem.Alias] = true
			}
		}
	}
	return nil
}

func validateSecretRefsPath(radixComponent radixv1.RadixCommonComponent) error {
	commonAzureKeyVaultPathMap := make(map[string]string)
	for _, azureKeyVault := range radixComponent.GetSecretRefs().AzureKeyVaults {
		path := azureKeyVault.Path
		if path != nil && len(*path) > 0 { // set only non-empty common path
			commonAzureKeyVaultPathMap[azureKeyVault.Name] = *path
		}
	}
	for _, environmentConfig := range radixComponent.GetEnvironmentConfig() {
		envAzureKeyVaultPathMap := make(map[string]string)
		for commonAzureKeyVaultName, path := range commonAzureKeyVaultPathMap {
			envAzureKeyVaultPathMap[commonAzureKeyVaultName] = path
		}
		for _, envAzureKeyVault := range environmentConfig.GetSecretRefs().AzureKeyVaults {
			if envAzureKeyVault.Path != nil && len(*envAzureKeyVault.Path) > 0 { // override common path by non-empty env-path, or set non-empty env path
				envAzureKeyVaultPathMap[envAzureKeyVault.Name] = *envAzureKeyVault.Path
			}
		}
		envPathMap := make(map[string]bool)
		for azureKeyVaultName, path := range envAzureKeyVaultPathMap {
			if _, existsForOtherKeyVault := envPathMap[path]; existsForOtherKeyVault {
				return fmt.Errorf("azure Key vault %s, component %s, path %s: %w", azureKeyVaultName, radixComponent.GetName(), path, ErrDuplicatePathForAzureKeyVault)
			}
			envPathMap[path] = true
		}
	}
	return nil
}

func getEnvVarNameMap(componentEnvVarsMap radixv1.EnvVarsMap, envsEnvVarsMap radixv1.EnvVarsMap) map[string]bool {
	envVarsMap := make(map[string]bool)
	for name := range componentEnvVarsMap {
		envVarsMap[name] = true
	}
	for name := range envsEnvVarsMap {
		envVarsMap[name] = true
	}
	return envVarsMap
}

func azureIdentityIsSet(commonComponent radixv1.RadixCommonComponent) bool {
	identity := commonComponent.GetIdentity()
	if identity != nil && identity.Azure != nil && identity.Azure.ClientId != "" {
		return true
	}
	for _, envConfig := range commonComponent.GetEnvironmentConfig() {
		if !commonComponent.GetEnabledForEnvironmentConfig(envConfig) {
			continue
		}
		envIdentity := envConfig.GetIdentity()
		if envIdentity != nil && envIdentity.Azure != nil && envIdentity.Azure.ClientId != "" {
			return true
		}
	}
	return false
}

func validateConflictingEnvironmentAndSecretRefsNames(component radixv1.RadixCommonComponent, envsEnvVarMap map[string]map[string]bool) error {
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		for _, item := range azureKeyVault.Items {
			for _, envVarMap := range envsEnvVarMap {
				if _, contains := envVarMap[item.EnvVar]; contains {
					return fmt.Errorf("component %s, envVar %s: %w", component.GetName(), item.EnvVar, ErrSecretRefEnvVarNameConflictsWithEnvironmentVariable)
				}
			}
		}
	}
	for _, environmentConfig := range component.GetEnvironmentConfig() {
		for _, azureKeyVault := range environmentConfig.GetSecretRefs().AzureKeyVaults {
			for _, item := range azureKeyVault.Items {
				if envVarMap, ok := envsEnvVarMap[environmentConfig.GetEnvironment()]; ok {
					if _, contains := envVarMap[item.EnvVar]; contains {
						return fmt.Errorf("component %s, envVar %s: %w", component.GetName(), item.EnvVar, ErrSecretRefEnvVarNameConflictsWithEnvironmentVariable)
					}
				}
			}
		}
	}
	return nil
}
