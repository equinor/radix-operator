package deployment

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func teardownSecretRefsTest() {
	// Cleanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.DeploymentsHistoryLimitEnvironmentVariable)
	os.Unsetenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	os.Unsetenv(defaults.OperatorClusterTypeEnvironmentVariable)
}

func TestSecretDeployed_SecretRefsCredentialsSecrets(t *testing.T) {
	defer teardownSecretRefsTest()
	appName, environment := "some-app", "dev"
	scenarios := []struct {
		componentName      string
		radixSecretRefs    v1.RadixSecretRefs
		expectedObjectName map[string]map[string]string
		componentIdentity  *v1.Identity
	}{
		{
			componentName: "componentName1",
			radixSecretRefs: v1.RadixSecretRefs{
				AzureKeyVaults: []v1.RadixAzureKeyVault{
					{
						Name: "azureKeyVaultName1",
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:   "secret1",
								EnvVar: "SECRET_REF1",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
							},
							{
								Name:   "key1",
								EnvVar: "KEY_REF1",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeKey),
							},
							{
								Name:   "cert1",
								EnvVar: "CERT_REF1",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeCert),
							},
						},
					},
				},
			},
		},
		{
			componentName: "componentName2",
			radixSecretRefs: v1.RadixSecretRefs{
				AzureKeyVaults: []v1.RadixAzureKeyVault{
					{
						Name: "azureKeyVaultName2",
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:   "secret1",
								EnvVar: "SECRET_REF1",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
							},
						},
					},
					{
						Name: "azureKeyVaultName3",
						Path: commonUtils.StringPtr("/mnt/kv2"),
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:   "secret1",
								EnvVar: "SECRET_REF11",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
							},
						},
					},
				},
			},
		},
		{
			componentName:   "componentName3",
			radixSecretRefs: v1.RadixSecretRefs{},
		},
		{
			componentName: "componentName4",
			radixSecretRefs: v1.RadixSecretRefs{
				AzureKeyVaults: []v1.RadixAzureKeyVault{
					{
						Name: "AZURE-KEY-VAULT4",
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:   "secret1",
								EnvVar: "SECRET_REF1",
								Type:   test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
							},
						},
					},
				},
			},
		},
		{
			componentName: "componentName5",
			radixSecretRefs: v1.RadixSecretRefs{
				AzureKeyVaults: []v1.RadixAzureKeyVault{
					{
						Name: "azureKeyVaultName5",
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:  "secret1",
								Type:  test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								Alias: commonUtils.StringPtr("some-secret1-alias1"),
							},
						},
					},
					{
						Name: "azureKeyVaultName6",
						Path: commonUtils.StringPtr("/mnt/kv2"),
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:  "secret1",
								Type:  test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								Alias: commonUtils.StringPtr("some-secret1-alias2"),
							},
						},
					},
				},
			},
			expectedObjectName: map[string]map[string]string{
				"azureKeyVaultName5": {
					"secret1": "some-secret1-alias1",
				},
				"azureKeyVaultName6": {
					"secret1": "some-secret1-alias2",
				},
			},
		},
		{
			componentName: "componentName6",
			radixSecretRefs: v1.RadixSecretRefs{
				AzureKeyVaults: []v1.RadixAzureKeyVault{
					{
						Name: "azureKeyVaultName5",
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:  "secret1",
								Type:  test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								Alias: commonUtils.StringPtr("some-secret1-alias1"),
							},
						},
						UseAzureIdentity: commonUtils.BoolPtr(true),
					},
					{
						Name: "azureKeyVaultName6",
						Path: commonUtils.StringPtr("/mnt/kv2"),
						Items: []v1.RadixAzureKeyVaultItem{
							{
								Name:  "secret1",
								Type:  test.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								Alias: commonUtils.StringPtr("some-secret1-alias2"),
							},
						},
						UseAzureIdentity: commonUtils.BoolPtr(true),
					},
				},
			},
			componentIdentity: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "some-client-id"}},
			expectedObjectName: map[string]map[string]string{
				"azureKeyVaultName5": {
					"secret1": "some-secret1-alias1",
				},
				"azureKeyVaultName6": {
					"secret1": "some-secret1-alias2",
				},
			},
		},
	}

	t.Run("get secret-refs Azure Key vaults credential secrets ", func(t *testing.T) {
		for _, scenario := range scenarios {
			testEnv := setupTestEnv()
			radixDeployment, err := applyRadixDeployWithSyncForSecretRefs(testEnv, appName, environment, scenario)

			assert.NoError(t, err)
			assert.NotNil(t, radixDeployment)

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)
			allSecrets, _ := testEnv.kubeclient.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
			secrets := test.GetAzureKeyVaultTypeSecrets(allSecrets)

			for _, azureKeyVault := range scenario.radixSecretRefs.AzureKeyVaults {
				credsSecretName := defaults.GetCsiAzureKeyVaultCredsSecretName(scenario.componentName, azureKeyVault.Name)
				if azureKeyVault.UseAzureIdentity != nil && *azureKeyVault.UseAzureIdentity {
					assert.False(t, secretByNameExists(credsSecretName, secrets), "Not expected credentials secret for Azure Key vault")
				} else {
					assert.True(t, secretByNameExists(credsSecretName, secrets), "Missing credentials secret for Azure Key vault")
				}
			}
		}
	})

	t.Run("get secret-refs secret provider class", func(t *testing.T) {
		for _, scenario := range scenarios {
			testEnv := setupTestEnv()
			rd, err := applyRadixDeployWithSyncForSecretRefs(testEnv, appName, environment, scenario)

			assert.NoError(t, err)
			assert.NotNil(t, rd)

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)
			allSecretProviderClasses, _ := testEnv.secretproviderclient.SecretsstoreV1().SecretProviderClasses(envNamespace).List(context.TODO(), metav1.ListOptions{})
			secretProviderClasses := allSecretProviderClasses.Items
			validatedSecretProviderClassesCount := 0

			assert.Equal(t, len(scenario.radixSecretRefs.AzureKeyVaults), len(secretProviderClasses))

			for _, azureKeyVault := range scenario.radixSecretRefs.AzureKeyVaults {
				expectedClassName := kube.GetComponentSecretProviderClassName(rd.Name, scenario.componentName, v1.RadixSecretRefTypeAzureKeyVault, azureKeyVault.Name)
				secretProviderClass := getSecretProviderClassByName(expectedClassName, secretProviderClasses)
				validatedSecretProviderClassesCount++
				assert.NotNil(t, secretProviderClass, "Missing secret provider class for Azure Key vault")
				usePodIdentitySecretParam, usePodIdentityParamExists := secretProviderClass.Spec.Parameters["usePodIdentity"]
				assert.True(t, usePodIdentityParamExists)
				assert.Equal(t, "false", usePodIdentitySecretParam)
				keyvaultNameSecretParam, keyvaultNameParamsExists := secretProviderClass.Spec.Parameters["keyvaultName"]
				assert.True(t, keyvaultNameParamsExists)
				assert.Equal(t, azureKeyVault.Name, keyvaultNameSecretParam)
				tenantIdSecretParam, tenantIdParamsExists := secretProviderClass.Spec.Parameters["tenantId"]
				assert.True(t, tenantIdParamsExists)
				assert.Equal(t, testTenantId, tenantIdSecretParam)
				objectsSecretParam, objectsSecretParamExists := secretProviderClass.Spec.Parameters["objects"]
				assert.True(t, objectsSecretParamExists)
				assert.True(t, len(objectsSecretParam) > 0)
				assert.Len(t, secretProviderClass.Spec.SecretObjects, 1)
				secretObject := secretProviderClass.Spec.SecretObjects[0]
				expectedSecretRefSecretName := kube.GetAzureKeyVaultSecretRefSecretName(scenario.componentName, rd.Name, azureKeyVault.Name, corev1.SecretTypeOpaque)
				assert.Equal(t, expectedSecretRefSecretName, secretObject.SecretName)
				assert.Equal(t, strings.ToLower(expectedSecretRefSecretName), secretObject.SecretName)
				assert.Equal(t, string(corev1.SecretTypeOpaque), secretObject.Type)
				assert.Equal(t, len(azureKeyVault.Items), len(secretObject.Data))
				secretObjectItemMap := make(map[string]*secretsstorev1.SecretObjectData)
				for _, item := range secretObject.Data {
					item := item
					secretObjectItemMap[item.Key] = item
				}
				for _, keyVaultItem := range azureKeyVault.Items {
					dataItem, exists := secretObjectItemMap[kube.GetSecretRefAzureKeyVaultItemDataKey(&keyVaultItem)]
					if !exists {
						assert.Fail(t, "missing data item for EnvVar or Alias")
					}
					if scenario.expectedObjectName != nil {
						assert.Equal(t, scenario.expectedObjectName[azureKeyVault.Name][keyVaultItem.Name],
							dataItem.ObjectName)
					} else {
						assert.Equal(t, keyVaultItem.Name, dataItem.ObjectName)
					}
					assert.True(t, strings.Contains(objectsSecretParam, dataItem.ObjectName))
				}
			}

			assert.Equal(t, validatedSecretProviderClassesCount, len(secretProviderClasses), "Not all secretProviderClasses where validated")
		}
	})
}

func Test_GetRadixComponentsForEnv_AzureKeyVault(t *testing.T) {
	type scenarioSpec struct {
		name                 string
		commonConfig         []v1.RadixAzureKeyVault
		configureEnvironment bool
		environmentConfig    []v1.RadixAzureKeyVault
		expected             []v1.RadixAzureKeyVault
	}

	scenarios := []scenarioSpec{
		{name: "empty when commonConfig is empty and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "empty when commonConfig is empty and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}}, configureEnvironment: true, environmentConfig: nil, expected: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}}},
		{name: "use commonConfig when environmentConfig.AzureKeyVaults is empty", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}}, configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{}}}, expected: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}}},
		{name: "override non-empty commonConfig with environmentConfig.AzureKeyVaults",
			commonConfig:         []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}},
			configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}},
			expected: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: nil, configureEnvironment: true,
			environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}},
			expected:          []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}}},
		{name: "override empty commonConfig.AzureKeyVaults with environmentConfig", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{}}},
			configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}},
			expected: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var1"}}}}},
		{name: "adds secret to non-empty commonConfig from environmentConfig.AzureKeyVaults",
			commonConfig:         []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}}},
			configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-2", EnvVar: "var2"}}}},
			expected: []v1.RadixAzureKeyVault{{Name: "key-vault-1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}, {Name: "secret-value-2", EnvVar: "var2"}}}}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			const envName = "any-env"
			component := utils.AnApplicationComponent().WithName("anycomponent").WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.commonConfig})
			if scenario.configureEnvironment {
				component = component.WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().WithEnvironment(envName).WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.environmentConfig}),
				)
			}
			ra := utils.ARadixApplication().WithComponents(component).BuildRA()
			sut := GetRadixComponentsForEnv
			components, err := sut(ra, envName, make(map[string]pipeline.ComponentImage), make(v1.EnvVarsMap))
			require.NoError(t, err)
			azureKeyVaults := components[0].SecretRefs.AzureKeyVaults
			assert.EqualValues(t, sortAzureKeyVaults(scenario.expected), sortAzureKeyVaults(azureKeyVaults))
		})
	}
}

func Test_GetRadixComponentsForEnv_AzureKeyVaultUseIAzureIdentity(t *testing.T) {
	type scenarioSpec struct {
		name                 string
		commonConfig         []v1.RadixAzureKeyVault
		configureEnvironment bool
		environmentConfig    []v1.RadixAzureKeyVault
		expected             *bool
	}

	createRadixAzureKeyVaultItem := func() []v1.RadixAzureKeyVaultItem {
		return []v1.RadixAzureKeyVaultItem{{Name: "secret-value-1", EnvVar: "var1"}}
	}

	scenarios := []scenarioSpec{
		{name: "empty when commonConfig is empty and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "empty when commonConfig is empty and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(true), Items: createRadixAzureKeyVaultItem()}}, configureEnvironment: true, environmentConfig: nil, expected: pointers.Ptr(true)},
		{name: "use commonConfig when environmentConfig.UseAzureIdentity is empty", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(true), Items: createRadixAzureKeyVaultItem()}}, configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: nil, Items: []v1.RadixAzureKeyVaultItem{}}}, expected: pointers.Ptr(true)},
		{name: "override non-empty commonConfig with environmentConfig.UseAzureIdentity",
			commonConfig:         []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(false), Items: createRadixAzureKeyVaultItem()}},
			configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(true), Items: createRadixAzureKeyVaultItem()}},
			expected: pointers.Ptr(true)},
		{name: "override empty commonConfig with environmentConfig", commonConfig: nil, configureEnvironment: true,
			environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(true), Items: createRadixAzureKeyVaultItem()}},
			expected:          pointers.Ptr(true)},
		{name: "override empty commonConfig.UseAzureIdentity with environmentConfig", commonConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: nil, Items: []v1.RadixAzureKeyVaultItem{}}},
			configureEnvironment: true, environmentConfig: []v1.RadixAzureKeyVault{{Name: "key-vault-1", UseAzureIdentity: pointers.Ptr(true), Items: createRadixAzureKeyVaultItem()}},
			expected: pointers.Ptr(true)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			const envName = "any-env"
			component := utils.AnApplicationComponent().WithName("anycomponent").WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.commonConfig})
			if scenario.configureEnvironment {
				component = component.WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().WithEnvironment(envName).WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.environmentConfig}),
				)
			}
			ra := utils.ARadixApplication().WithComponents(component).BuildRA()
			sut := GetRadixComponentsForEnv
			components, err := sut(ra, envName, make(map[string]pipeline.ComponentImage), make(v1.EnvVarsMap))
			require.NoError(t, err)
			azureKeyVaults := components[0].SecretRefs.AzureKeyVaults
			if len(azureKeyVaults) == 0 {
				assert.Nil(t, scenario.expected)
			} else {
				assert.Equal(t, scenario.expected, azureKeyVaults[0].UseAzureIdentity)
			}
		})
	}
}

func sortAzureKeyVaults(azureKeyVaults []v1.RadixAzureKeyVault) []v1.RadixAzureKeyVault {
	sort.Slice(azureKeyVaults, func(i, j int) bool {
		if len(azureKeyVaults) == 0 {
			return false
		}
		x := azureKeyVaults[i]
		y := azureKeyVaults[j]
		return x.Name < y.Name
	})
	for _, azureKeyVault := range azureKeyVaults {
		azureKeyVault.Items = sortAzureKeyVaultItems(azureKeyVault.Items)
	}
	return azureKeyVaults
}
func sortAzureKeyVaultItems(items []v1.RadixAzureKeyVaultItem) []v1.RadixAzureKeyVaultItem {
	sort.Slice(items, func(i, j int) bool {
		if len(items) == 0 {
			return false
		}
		x := items[i]
		y := items[j]
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		if x.EnvVar != y.EnvVar {
			return x.EnvVar < y.EnvVar
		}
		if x.Alias == nil {
			return false
		}
		if y.Alias == nil {
			return true
		}
		return *(x.Alias) < *(y.Alias)
	})
	return items
}

func getSecretProviderClassByName(className string, classes []secretsstorev1.SecretProviderClass) *secretsstorev1.SecretProviderClass {
	for _, secretProviderClass := range classes {
		if secretProviderClass.GetName() == className {
			return &secretProviderClass
		}
	}
	return nil
}

func applyRadixDeployWithSyncForSecretRefs(testEnv *testEnvProps, appName string, environment string, scenario struct {
	componentName      string
	radixSecretRefs    v1.RadixSecretRefs
	expectedObjectName map[string]map[string]string
	componentIdentity  *v1.Identity
}) (*v1.RadixDeployment, error) {
	radixDeployment, err := applyDeploymentWithSyncForTestEnv(testEnv, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(scenario.componentName).
				WithPort("http", 8080).
				WithSecretRefs(scenario.radixSecretRefs).
				WithIdentity(scenario.componentIdentity),
		),
	)
	return radixDeployment, err
}
