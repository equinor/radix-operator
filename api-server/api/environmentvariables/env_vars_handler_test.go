package environmentvariables

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	envvarsmodels "github.com/equinor/radix-operator/api-server/api/environmentvariables/models"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetEnvVars(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, environmentName)
	t.Run("Get existing env vars", func(t *testing.T) {
		t.Parallel()
		kubeClient, radixClient, _, promClient, commonTestUtils, kubeUtil, _, certClient := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		}
		err := setupDeployment(&commonTestUtils, kubeClient, radixClient, promClient, certClient, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap).
				WithSecrets([]string{"SECRET1", "SECRET2"})
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{UserAccount: models.Account{Client: kubeClient}},
		}

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(context.Background(), appName, environmentName, componentName)

		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		envVarMap := convertToMap(envVars)
		assert.Equal(t, "val1", envVarMap["VAR1"], "Invalid or missing VAR1")
		assert.Equal(t, "val2", envVarMap["VAR2"], "Invalid or missing VAR2")
		_, existsSecret1 := envVarMap["SECRET1"]
		assert.False(t, existsSecret1, "Unexpected secret SECRET1 among env-vars")
		_, existsSecret2 := envVarMap["SECRET2"]
		assert.False(t, existsSecret2, "Unexpected secret SECRET2 among env-vars")
		assertRadixEnvVars(t, envVarMap)
	})
}

func Test_ChangeGetEnvVars(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, environmentName)
	t.Run("Change existing env var", func(t *testing.T) {
		t.Parallel()
		kubeClient, radixClient, _, promClient, commonTestUtils, kubeUtil, _, certClient := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
			"VAR3": "val3",
		}
		err := setupDeployment(&commonTestUtils, kubeClient, radixClient, promClient, certClient, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap)
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{UserAccount: models.Account{Client: kubeClient}},
		}

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		params := []envvarsmodels.EnvVarParameter{
			{
				Name:  "VAR2",
				Value: "new-val2",
			},
			{
				Name:  "VAR3",
				Value: "new-val3",
			},
		}
		err = handler.ChangeEnvVar(context.Background(), appName, environmentName, componentName, params)

		assert.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(context.Background(), appName, environmentName, componentName)
		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		envVarMap := convertToMap(envVars)
		assert.Equal(t, "val1", envVarMap["VAR1"], "Invalid or missing VAR1")
		assert.Equal(t, "new-val2", envVarMap["VAR2"], "Invalid or missing VAR2")
		assert.Equal(t, "new-val3", envVarMap["VAR3"], "Invalid or missing VAR3")
		assertRadixEnvVars(t, envVarMap)
	})
	t.Run("Skipped changing not-existing env vars", func(t *testing.T) {
		t.Parallel()
		kubeClient, radixClient, _, promClient, commonTestUtils, kubeUtil, _, certClient := setupTest(t)

		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		}
		err := setupDeployment(&commonTestUtils, kubeClient, radixClient, promClient, certClient, appName, environmentName, componentName, func(builder operatorutils.DeployComponentBuilder) {
			builder.WithEnvironmentVariables(envVarsMap)
		})
		require.NoError(t, err)
		handler := envVarsHandler{
			kubeUtil:        commonTestUtils.GetKubeUtil(),
			inClusterClient: nil,
			accounts:        models.Accounts{UserAccount: models.Account{Client: kubeClient}},
		}

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsConfigMapName(componentName))
		require.NoError(t, err)

		_, err = kubeUtil.GetConfigMap(context.Background(), namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
		require.NoError(t, err)

		params := []envvarsmodels.EnvVarParameter{
			{
				Name:  "SOME_NOT_EXISTING_VAR",
				Value: "new-val",
			},
			{
				Name:  "VAR2",
				Value: "new-val2",
			},
		}
		err = handler.ChangeEnvVar(context.Background(), appName, environmentName, componentName, params)

		require.NoError(t, err)

		envVars, err := handler.GetComponentEnvVars(context.Background(), appName, environmentName, componentName)
		assert.NoError(t, err)
		assert.NotEmpty(t, envVars)
		envVarMap := convertToMap(envVars)
		assert.Equal(t, "val1", envVarMap["VAR1"], "Invalid or missing VAR1")
		assert.Equal(t, "new-val2", envVarMap["VAR2"], "Invalid or missing VAR2")
		assertRadixEnvVars(t, envVarMap)
	})
}

func convertToMap(envVars []envvarsmodels.EnvVar) map[string]string {
	return slice.Reduce(envVars, make(map[string]string), func(acc map[string]string, envVar envvarsmodels.EnvVar) map[string]string {
		acc[envVar.Name] = envVar.Value
		return acc
	})
}

func assertRadixEnvVars(t *testing.T, envVarMap map[string]string) {
	assert.Equal(t, appName, envVarMap[defaults.RadixAppEnvironmentVariable], "Invalid or missing RADIX_APP env-var")
	assert.Equal(t, environmentName, envVarMap[defaults.EnvironmentnameEnvironmentVariable], "Invalid or missing RADIX_ENVIRONMENT env-var")
	assert.Equal(t, clusterName, envVarMap[defaults.ClusternameEnvironmentVariable], "Invalid or missing RADIX_CLUSTERNAME env-var")
	assert.Equal(t, clusterType, envVarMap[defaults.RadixClusterTypeEnvironmentVariable], "Invalid or missing RADIX_CLUSTER_TYPE env-var")
	assert.Equal(t, componentName, envVarMap[defaults.RadixComponentEnvironmentVariable], "Invalid or missing RADIX_COMPONENT env-var")
	assert.Equal(t, "any.container.registry", envVarMap[defaults.ContainerRegistryEnvironmentVariable], "Invalid or missing RADIX_CONTAINER_REGISTRY env-var")
	assert.Equal(t, "dev.radix.equinor.com", envVarMap[defaults.RadixDNSZoneEnvironmentVariable], "Invalid or missing RADIX_DNS_ZONE env-var")
}
