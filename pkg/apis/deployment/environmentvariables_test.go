package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Test_order_of_env_variables(t *testing.T) {
	data := map[string]string{
		"d_key": "4",
		"a_key": "1",
		"c_key": "3",
		"g_key": "6",
		"b_key": "2",
		"q_key": "7",
		"e_key": "5",
	}
	envVarsConfigMap := &corev1.ConfigMap{Data: data}

	envVars := getEnvVars(envVarsConfigMap, nil)
	assert.Len(t, envVars, len(data))
	assert.Equal(t, "a_key", envVars[0].Name)
	assert.Equal(t, "b_key", envVars[1].Name)
	assert.Equal(t, "c_key", envVars[2].Name)
	assert.Equal(t, "d_key", envVars[3].Name)
	assert.Equal(t, "e_key", envVars[4].Name)
	assert.Equal(t, "g_key", envVars[5].Name)
	assert.Equal(t, "q_key", envVars[6].Name)
	for _, envVar := range envVars {
		assert.Equal(t, data[envVar.Name], envVarsConfigMap.Data[envVar.Name])
	}
}

func Test_GetEnvironmentVariables(t *testing.T) {
	appName := "any-app"
	envName := "dev"
	componentName := "any-component"
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	defer teardownTest()

	t.Run("Get env vars", func(t *testing.T) {
		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
			"VAR3": "val3",
		}
		rd := applyRd(t, appName, envName, componentName, envVarsMap, tu, client, kubeUtil, radixclient, prometheusclient)

		envVars, err := GetEnvironmentVariables(kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.Len(t, envVars, 3)
		envVarsConfigMap, envVarsConfigMapMetadata, err := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
		assert.NoError(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Equal(t, "val1", envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "val2", envVarsConfigMap.Data["VAR2"])
		assert.Equal(t, "val3", envVarsConfigMap.Data["VAR3"])
		assert.NotNil(t, envVarsConfigMapMetadata)
	})
}

func Test_getEnvironmentVariablesForRadixOperator(t *testing.T) {
	appName := "any-app"
	envName := "dev"
	componentName := "any-component"
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	defer teardownTest()

	t.Run("Get env vars", func(t *testing.T) {
		envVarsMap := map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
			"VAR3": "val3",
		}
		rd := applyRd(t, appName, envName, componentName, envVarsMap, tu, client, kubeUtil, radixclient, prometheusclient)
		kubeUtil.CreateConfigMap(corev1.NamespaceDefault, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "radix-config"}, Data: map[string]string{
			"clustername":       clusterName,
			"containerRegistry": anyContainerRegistry,
		}})

		envVars, err := getEnvironmentVariablesForRadixOperator(kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.True(t, len(envVars) > 3)
		envVarsConfigMap, envVarsConfigMapMetadata, err := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
		assert.NoError(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Equal(t, "val1", envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "val2", envVarsConfigMap.Data["VAR2"])
		assert.Equal(t, "val3", envVarsConfigMap.Data["VAR3"])
		resultEnvVarsMap := map[string]corev1.EnvVar{}
		for _, envVar := range envVars {
			envVar := envVar
			resultEnvVarsMap[envVar.Name] = envVar
		}
		assert.Equal(t, anyContainerRegistry, resultEnvVarsMap["RADIX_CONTAINER_REGISTRY"].Value)
		assert.Equal(t, clusterName, resultEnvVarsMap["RADIX_CLUSTERNAME"].Value)
		assert.NotNil(t, envVarsConfigMapMetadata)
	})
}

func Test_RemoveFromConfigMapEnvVarsNotExistingInRadixDeployment(t *testing.T) {
	appName := "any-app"
	envName := "dev"
	namespace := utils.GetEnvironmentNamespace(appName, env)
	componentName := "any-component"
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	defer teardownTest()
	t.Run("Remove obsolete env-vars from config-maps", func(t *testing.T) {
		kubeUtil.CreateConfigMap(namespace, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: kube.GetEnvVarsConfigMapName(componentName)}, Data: map[string]string{
			"VAR1":          "val1",
			"OUTDATED_VAR1": "val1z",
		}})
		existingEnvVarsMetadataConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: kube.GetEnvVarsMetadataConfigMapName(componentName)}}
		kube.SetEnvVarsMetadataMapToConfigMap(&existingEnvVarsMetadataConfigMap,
			map[string]kube.EnvVarMetadata{
				"VAR1":          {RadixConfigValue: "orig-val1"},
				"OUTDATED_VAR1": {RadixConfigValue: "orig-val1a"},
				"OUTDATED_VAR2": {RadixConfigValue: "orig-val2a"},
			})
		kubeUtil.CreateConfigMap(namespace, &existingEnvVarsMetadataConfigMap)

		radixConfigEnvVars := map[string]string{
			"VAR1": "new-val1",
			"VAR2": "val2",
			"VAR3": "val3",
		}

		rd := applyRd(t, appName, envName, componentName, radixConfigEnvVars, tu, client, kubeUtil, radixclient, prometheusclient)
		envVars, err := GetEnvironmentVariables(kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.Len(t, envVars, 3)
		envVarsConfigMap, envVarsMetadataConfigMap, err := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
		assert.NoError(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Len(t, envVarsConfigMap.Data, 3)
		assert.Equal(t, "new-val1", envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "val2", envVarsConfigMap.Data["VAR2"])
		assert.Equal(t, "val3", envVarsConfigMap.Data["VAR3"])
		assert.Equal(t, "", envVarsConfigMap.Data["OUTDATED_VAR1"])
		assert.NotNil(t, envVarsMetadataConfigMap)
		assert.NotNil(t, envVarsMetadataConfigMap.Data)
		resultEnvVarsMetadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(envVarsMetadataConfigMap)
		assert.NoError(t, err)
		assert.NotNil(t, resultEnvVarsMetadataMap)
		assert.Len(t, resultEnvVarsMetadataMap, 1)
		assert.NotEmpty(t, resultEnvVarsMetadataMap["VAR1"])
		assert.Equal(t, "new-val1", resultEnvVarsMetadataMap["VAR1"].RadixConfigValue)
		assert.Empty(t, resultEnvVarsMetadataMap["OUTDATED_VAR1"])
		assert.Empty(t, resultEnvVarsMetadataMap["OUTDATED_VAR2"])
	})
}

func applyRd(t *testing.T, appName string, envName string, componentName string, envVarsMap map[string]string, tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, prometheusclient monitoring.Interface) *v1.RadixDeployment {
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(envName).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithEnvironmentVariables(envVarsMap))

	rd, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	assert.NoError(t, err)
	return rd
}
