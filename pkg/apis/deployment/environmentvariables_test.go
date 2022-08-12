package deployment

import (
	"strings"
	"testing"

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
	testEnv := setupTestEnv()
	defer teardownTest()

	t.Run("Get env vars", func(t *testing.T) {
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
	testEnv := setupTestEnv()
	defer teardownTest()

	t.Run("Get env vars", func(t *testing.T) {
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
	testEnv := setupTestEnv()
	defer teardownTest()
	t.Run("Remove obsolete env-vars from config-maps", func(t *testing.T) {
		config_map := kube.BuildRadixConfigEnvVarsConfigMap(appName, componentName)
		config_map.Data = map[string]string{
			"VAR1":          "val1",
			"OUTDATED_VAR1": "val1z",
		}
		//goland:noinspection GoUnhandledErrorResult
		testEnv.kubeUtil.CreateConfigMap(namespace, config_map)

		existingEnvVarsMetadataConfigMap := kube.BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName)
		//goland:noinspection GoUnhandledErrorResult
		kube.SetEnvVarsMetadataMapToConfigMap(existingEnvVarsMetadataConfigMap,
			map[string]kube.EnvVarMetadata{
				"VAR1":          {RadixConfigValue: "orig-val1"},
				"OUTDATED_VAR1": {RadixConfigValue: "orig-val1a"},
				"OUTDATED_VAR2": {RadixConfigValue: "orig-val2a"},
			})
		//goland:noinspection GoUnhandledErrorResult
		testEnv.kubeUtil.CreateConfigMap(namespace, existingEnvVarsMetadataConfigMap)

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
	scenarios := []struct {
		name                      string
		secrets                   []string
		envVars                   map[string]string
		azureKeyVaults            []v1.RadixAzureKeyVault
		expectedEnvVars           map[string]string
		expectedSecrets           []string
		expectedSecretRefsEnvVars []string
	}{{
		name: "Env vars with one Azure Key Vault of different types",
		envVars: map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		},
		secrets: []string{"SECRET1", "SECRET2"},
		azureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: "kv1",
				Items: []v1.RadixAzureKeyVaultItem{
					{Name: "secret1", EnvVar: "SECRET_REF_1"},
					{Name: "secret1", EnvVar: "SECRET_REF_2"},
					{Name: "secret3", EnvVar: "SECRET_REF_3", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret)},
					{Name: "key1", EnvVar: "KEY_REF_1", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeKey)},
					{Name: "cert1", EnvVar: "CERT_REF_1", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeCert)},
				},
			},
		},
		expectedEnvVars: map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		},
		expectedSecrets: []string{"SECRET1", "SECRET2"},
		expectedSecretRefsEnvVars: []string{
			"SECRET_REF_1",
			"SECRET_REF_2",
			"SECRET_REF_3",
			"KEY_REF_1",
			"CERT_REF_1",
		},
	},
		{
			name: "Multiple Azure Key Vault of different types",
			azureKeyVaults: []v1.RadixAzureKeyVault{
				{
					Name: "kv1",
					Items: []v1.RadixAzureKeyVaultItem{
						{Name: "secret1", EnvVar: "SECRET_REF_1"},
						{Name: "secret1", EnvVar: "SECRET_REF_2"},
						{Name: "secret3", EnvVar: "SECRET_REF_3", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret)},
						{Name: "key1", EnvVar: "KEY_REF_1", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeKey)},
						{Name: "cert1", EnvVar: "CERT_REF_1", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeCert)},
					},
				},
				{
					Name: "kv2",
					Items: []v1.RadixAzureKeyVaultItem{
						{Name: "secret1", EnvVar: "SECRET_REF_21"},
						{Name: "secret1", EnvVar: "SECRET_REF_22"},
						{Name: "secret3", EnvVar: "SECRET_REF_23", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret)},
						{Name: "key1", EnvVar: "KEY_REF_21", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeKey)},
						{Name: "cert1", EnvVar: "CERT_REF_21", Type: test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeCert)},
					},
				},
			},
			expectedSecretRefsEnvVars: []string{
				"SECRET_REF_1",
				"SECRET_REF_2",
				"SECRET_REF_3",
				"KEY_REF_1",
				"CERT_REF_1",
				"SECRET_REF_21",
				"SECRET_REF_22",
				"SECRET_REF_23",
				"KEY_REF_21",
				"CERT_REF_21",
			},
		},
	}

	t.Run("Get env vars, secrets and secret-refs", func(t *testing.T) {
		t.Parallel()
		for _, testCase := range scenarios {
			t.Logf("Test case: %s", testCase.name)
			testEnv := setupTestEnv()
			rd := testEnv.applyRd(t, appName, envName, componentName, func(componentBuilder *utils.DeployComponentBuilder) {
				(*componentBuilder).
					WithEnvironmentVariables(testCase.envVars).
					WithSecrets(testCase.secrets).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: testCase.azureKeyVaults})
			})

			envVars, err := GetEnvironmentVariables(testEnv.kubeUtil, appName, rd, &rd.Spec.Components[0])

			resultEnvVarMap := make(map[string]corev1.EnvVar)
			for _, envVar := range envVars {
				resultEnvVarMap[envVar.Name] = envVar
			}

			assert.NoError(t, err)
			assert.Len(t, envVars, len(testCase.expectedEnvVars)+len(testCase.expectedSecrets)+len(testCase.expectedSecretRefsEnvVars))
			envVarsConfigMap, envVarsConfigMapMetadata, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(utils.GetEnvironmentNamespace(appName, env), appName, componentName)
			assert.NoError(t, err)
			assert.NotNil(t, envVarsConfigMap)
			assert.NotNil(t, envVarsConfigMapMetadata)
			assert.Equal(t, appName, envVarsConfigMap.ObjectMeta.Labels[kube.RadixAppLabel])
			assert.Equal(t, appName, envVarsConfigMapMetadata.ObjectMeta.Labels[kube.RadixAppLabel])
			assert.NotNil(t, envVarsConfigMap.Data)
			testedResultEnvVars := make(map[string]bool)
			for envVarName, envVarVal := range testCase.expectedEnvVars {
				assert.Equal(t, envVarVal, envVarsConfigMap.Data[envVarName])
				envVar, ok := resultEnvVarMap[envVarName]
				assert.True(t, ok)
				assert.NotNil(t, envVar.ValueFrom)
				assert.NotNil(t, envVar.ValueFrom.ConfigMapKeyRef)
				assert.Equal(t, envVarName, envVar.ValueFrom.ConfigMapKeyRef.Key)
				assert.True(t, strings.HasPrefix(envVar.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, "env-vars-"))
				assert.True(t, strings.HasSuffix(envVar.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, componentName))
				testedResultEnvVars[envVarName] = true
			}
			for _, secretName := range testCase.expectedSecrets {
				envVar, ok := resultEnvVarMap[secretName]
				assert.True(t, ok)
				assert.NotNil(t, envVar.ValueFrom)
				assert.NotNil(t, envVar.ValueFrom.SecretKeyRef)
				assert.Equal(t, secretName, envVar.ValueFrom.SecretKeyRef.Key)
				assert.True(t, strings.HasPrefix(envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name, componentName))
				testedResultEnvVars[secretName] = true
			}
			for _, secretRefsEnvVar := range testCase.expectedSecretRefsEnvVars {
				envVar, ok := resultEnvVarMap[secretRefsEnvVar]
				assert.True(t, ok)
				assert.NotNil(t, envVar.ValueFrom)
				assert.NotNil(t, envVar.ValueFrom.SecretKeyRef)
				assert.Equal(t, secretRefsEnvVar, envVar.ValueFrom.SecretKeyRef.Key)
				assert.True(t, strings.HasPrefix(envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name, componentName))
				assert.True(t, strings.Contains(envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name, "-az-keyvault-"))
				testedResultEnvVars[secretRefsEnvVar] = true
			}
			var notTestedEnvVars []string
			for envVarName := range resultEnvVarMap {
				if _, tested := testedResultEnvVars[envVarName]; !tested {
					notTestedEnvVars = append(notTestedEnvVars, envVarName)
				}
			}
			if len(notTestedEnvVars) > 0 {
				assert.Fail(t, "Not expected env-vars and/or secrets:", strings.Join(notTestedEnvVars, ", "))
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

func setupTestEnv() *testEnvProps {
	testEnv := testEnvProps{}
	testEnv.testUtil, testEnv.kubeclient, testEnv.kubeUtil, testEnv.radixclient, testEnv.prometheusclient, testEnv.secretproviderclient = setupTest()
	return &testEnv
}
