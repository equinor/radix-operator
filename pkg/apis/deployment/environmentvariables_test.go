package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	"strings"
	"testing"
)

type testEnvProps struct {
	kubeclient           kubernetes.Interface
	radixclient          radixclient.Interface
	secretproviderclient secretProviderClient.Interface
	prometheusclient     prometheusclient.Interface
	kubeUtil             *kube.Kube
	testUtil             *test.Utils
}

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
	testEnv := setupTextEnv()

	t.Run("Get env vars", func(t *testing.T) {
		t.Parallel()

		rd := testEnv.applyRd(t, appName, envName, componentName, func(componentBuilder *utils.DeployComponentBuilder) {
			(*componentBuilder).WithEnvironmentVariables(map[string]string{
				"VAR1": "val1",
				"VAR2": "val2",
				"VAR3": "val3",
			})
		})

		envVars, err := GetEnvironmentVariables(testEnv.kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.Len(t, envVars, 3)
		envVarsConfigMap, envVarsConfigMapMetadata, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
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
	testEnv := setupTextEnv()

	t.Run("Get env vars", func(t *testing.T) {
		t.Parallel()

		rd := testEnv.applyRd(t, appName, envName, componentName, func(componentBuilder *utils.DeployComponentBuilder) {
			(*componentBuilder).WithEnvironmentVariables(map[string]string{
				"VAR1": "val1",
				"VAR2": "val2",
				"VAR3": "val3",
			})
		})
		//goland:noinspection GoUnhandledErrorResult
		testEnv.kubeUtil.CreateConfigMap(corev1.NamespaceDefault, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "radix-config"}, Data: map[string]string{
			"clustername":       clusterName,
			"containerRegistry": anyContainerRegistry,
		}})

		envVars, err := getEnvironmentVariablesForRadixOperator(testEnv.kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.True(t, len(envVars) > 3)
		envVarsConfigMap, envVarsConfigMapMetadata, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
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
	testEnv := setupTextEnv()
	t.Run("Remove obsolete env-vars from config-maps", func(t *testing.T) {
		t.Parallel()

		//goland:noinspection GoUnhandledErrorResult
		testEnv.kubeUtil.CreateConfigMap(namespace, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: kube.GetEnvVarsConfigMapName(componentName)}, Data: map[string]string{
			"VAR1":          "val1",
			"OUTDATED_VAR1": "val1z",
		}})
		existingEnvVarsMetadataConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: kube.GetEnvVarsMetadataConfigMapName(componentName)}}
		//goland:noinspection GoUnhandledErrorResult
		kube.SetEnvVarsMetadataMapToConfigMap(&existingEnvVarsMetadataConfigMap,
			map[string]kube.EnvVarMetadata{
				"VAR1":          {RadixConfigValue: "orig-val1"},
				"OUTDATED_VAR1": {RadixConfigValue: "orig-val1a"},
				"OUTDATED_VAR2": {RadixConfigValue: "orig-val2a"},
			})
		//goland:noinspection GoUnhandledErrorResult
		testEnv.kubeUtil.CreateConfigMap(namespace, &existingEnvVarsMetadataConfigMap)

		rd := testEnv.applyRd(t, appName, envName, componentName, func(componentBuilder *utils.DeployComponentBuilder) {
			(*componentBuilder).WithEnvironmentVariables(map[string]string{
				"VAR1": "new-val1",
				"VAR2": "val2",
				"VAR3": "val3",
			})
		})
		envVars, err := GetEnvironmentVariables(testEnv.kubeUtil, appName, rd, &rd.Spec.Components[0])

		assert.NoError(t, err)
		assert.Len(t, envVars, 3)
		envVarsConfigMap, envVarsMetadataConfigMap, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
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

func Test_GetRadixSecretRefsAsEnvironmentVariables(t *testing.T) {
	appName := "any-app"
	envName := "dev"
	componentName := "any-component"
	testCase := []struct {
		envVars                   map[string]string
		azureKeyVaults            []v1.RadixAzureKeyVault
		expectedEnvVars           map[string]string
		expectedSecretRefsEnvVars []string
	}{{
		envVars: map[string]string{"VAR1": "val1"},
		azureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: "kv1",
				Items: []v1.RadixAzureKeyVaultItem{
					{Name: "secret1", EnvVar: "SECRET_REF_1"},
				},
			},
		},
		expectedEnvVars: map[string]string{
			"VAR1": "val1",
		},
		expectedSecretRefsEnvVars: []string{
			"SECRET_REF_1",
		},
	}}

	t.Run("Get env vars and secret-refs", func(t *testing.T) {
		t.Parallel()
		for _, test := range testCase {
			testEnv := setupTextEnv()
			rd := testEnv.applyRd(t, appName, envName, componentName, func(componentBuilder *utils.DeployComponentBuilder) {
				(*componentBuilder).
					WithEnvironmentVariables(test.envVars).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: test.azureKeyVaults})
			})

			envVars, err := GetEnvironmentVariables(testEnv.kubeUtil, appName, rd, &rd.Spec.Components[0])

			resultEnvVarMap := make(map[string]corev1.EnvVar)
			for _, envVar := range envVars {
				resultEnvVarMap[envVar.Name] = envVar
			}

			assert.NoError(t, err)
			assert.Len(t, envVars, len(test.expectedEnvVars)+len(test.expectedSecretRefsEnvVars))
			envVarsConfigMap, envVarsConfigMapMetadata, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
			assert.NoError(t, err)
			assert.NotNil(t, envVarsConfigMap)
			assert.NotNil(t, envVarsConfigMapMetadata)
			assert.NotNil(t, envVarsConfigMap.Data)
			for envVarName, envVarVal := range test.expectedEnvVars {
				assert.Equal(t, envVarVal, envVarsConfigMap.Data[envVarName])
				envVar, ok := resultEnvVarMap[envVarName]
				assert.True(t, ok)
				assert.NotNil(t, envVar.ValueFrom)
				assert.NotNil(t, envVar.ValueFrom.ConfigMapKeyRef)
				assert.Equal(t, envVarName, envVar.ValueFrom.ConfigMapKeyRef.Key)
				assert.True(t, strings.HasPrefix(envVar.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, "env-vars-"))
				assert.True(t, strings.HasSuffix(envVar.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, componentName))
			}
			for _, envVarName := range test.expectedSecretRefsEnvVars {
				envVar, ok := resultEnvVarMap[envVarName]
				assert.True(t, ok)
				assert.NotNil(t, envVar.ValueFrom)
				assert.NotNil(t, envVar.ValueFrom.SecretKeyRef)
				assert.Equal(t, envVarName, envVar.ValueFrom.SecretKeyRef.Key)
				assert.True(t, strings.HasPrefix(envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name, componentName))
				assert.True(t, strings.Contains(envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name, "-az-keyvault-"))
			}
		}
	})
}

func (testEnv *testEnvProps) applyRd(t *testing.T, appName string, envName string, componentName string, modify func(componentBuilder *utils.DeployComponentBuilder)) *v1.RadixDeployment {
	componentBuilder := utils.NewDeployComponentBuilder().
		WithName(componentName).
		WithPort("http", 8080).
		WithPublicPort("http")
	modify(&componentBuilder)
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(envName).
		WithEmptyStatus().
		WithComponents(componentBuilder)

	rd, err := applyDeploymentWithSync(testEnv.testUtil, testEnv.kubeclient, testEnv.kubeUtil, testEnv.radixclient, testEnv.prometheusclient, radixDeployBuilder)
	assert.NoError(t, err)
	return rd
}

func setupTextEnv() *testEnvProps {
	testEnv := testEnvProps{}
	testEnv.testUtil, testEnv.kubeclient, testEnv.kubeUtil, testEnv.radixclient, testEnv.prometheusclient, testEnv.secretproviderclient = setupTest()
	return &testEnv
}
