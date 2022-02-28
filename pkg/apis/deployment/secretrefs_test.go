package deployment

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	corev1 "k8s.io/api/core/v1"
	"os"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				assert.True(t, secretByNameExists(credsSecretName, secrets), "Missing credentials secret for Azure Key vault")
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
					dataItem, exists := secretObjectItemMap[getSecretRefAzureKeyVaultItemDataKey(&keyVaultItem)]
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
}) (*v1.RadixDeployment, error) {
	radixDeployment, err := applyDeploymentWithSyncForTestEnv(testEnv, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(scenario.componentName).
				WithPort("http", 8080).
				WithSecretRefs(scenario.radixSecretRefs),
		),
	)
	return radixDeployment, err
}
