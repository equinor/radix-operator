package deployment

import (
	"context"
	"os"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupSecretRefsTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, prometheusclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, anyContainerRegistry, egressIps)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient, prometheusclient
}

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
	// Setup
	testUtils, client, kubeUtil, radixclient, prometheusclient := setupSecretRefsTest()
	appName, environment := "some-app", "dev"
	componentName1, componentName2, componentName3, componentName4 := "component1", "component2", "component3", "component4"
	azureKeyVaultName1, azureKeyVaultName2, azureKeyVaultName3 := "keyvault1", "keyvault2", "keyvault3"

	radixDeployment, err := applyDeploymentWithSync(testUtils, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1).
				WithPort("http", 8080).
				WithSecretRefs(v1.RadixSecretRefs{
					AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: azureKeyVaultName1,
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret1",
									EnvVar: "SECRET_REF1",
									Type:   utils.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								},
								{
									Name:   "key1",
									EnvVar: "KEY_REF1",
									Type:   utils.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeKey),
								},
								{
									Name:   "cert1",
									EnvVar: "CERT_REF1",
									Type:   utils.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeCert),
								},
							},
						},
					},
				},
				),
			utils.NewDeployComponentBuilder().
				WithName(componentName2).
				WithPort("http", 8081).
				WithSecretRefs(v1.RadixSecretRefs{
					AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: azureKeyVaultName1,
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret1",
									EnvVar: "SECRET_REF1",
									Type:   utils.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								},
							},
						},
						{
							Name: azureKeyVaultName2,
							Path: commonUtils.StringPtr("/mnt/kv2"),
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret1",
									EnvVar: "SECRET_REF11",
									Type:   utils.GetRadixAzureKeyVaultObjectTypePtr(v1.RadixAzureKeyVaultObjectTypeSecret),
								},
							},
						},
					},
				},
				),
			utils.NewDeployComponentBuilder().
				WithName(componentName3).
				WithPort("http", 8082).
				WithSecretRefs(v1.RadixSecretRefs{
					AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: azureKeyVaultName3,
						},
					},
				}),
			utils.NewDeployComponentBuilder().
				WithName(componentName4).
				WithPort("http", 8082),
		),
	)

	assert.NoError(t, err)
	assert.NotNil(t, radixDeployment)

	t.Run("get secret-refs Azure Key vaults credential secrets ", func(t *testing.T) {
		var secretName string

		envNamespace := utils.GetEnvironmentNamespace(appName, environment)
		allSecrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
		secrets := utils.GetAzureKeyVaultTypeSecrets(allSecrets)
		assert.Equal(t, 4, len(secrets.Items), "Number of secrets was not according to spec")

		secretName = defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1)
		assert.True(t, secretByNameExists(secretName, secrets))
		secretName = defaults.GetCsiAzureKeyVaultCredsSecretName(componentName2, azureKeyVaultName1)
		assert.True(t, secretByNameExists(secretName, secrets))
		secretName = defaults.GetCsiAzureKeyVaultCredsSecretName(componentName2, azureKeyVaultName2)
		assert.True(t, secretByNameExists(secretName, secrets))
		secretName = defaults.GetCsiAzureKeyVaultCredsSecretName(componentName3, azureKeyVaultName3)
		assert.True(t, secretByNameExists(secretName, secrets), "Missing credentials secret for Azure Key vault config with empty Items")
	})

	teardownSecretRefsTest()
}
