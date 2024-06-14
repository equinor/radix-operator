package deployment

import (
	"context"
	"os"
	"testing"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupSecretsTest(t *testing.T) (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, kedav2.Interface, prometheusclient.Interface, *certfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient, kedaClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites(testClusterName, testEgressIps, "anysubid")
	require.NoError(t, err)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient
}

func teardownSecretsTest() {
	// Cleanup setup
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	_ = os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	_ = os.Unsetenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	_ = os.Unsetenv(defaults.OperatorClusterTypeEnvironmentVariable)
}

func TestSecretDeployed_ClientCertificateSecretGetsSet(t *testing.T) {
	// Setup
	testUtils, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient := setupSecretsTest(t)
	appName, environment := "edcradix", "test"
	componentName1, componentName2, componentName3, componentName4 := "component1", "component2", "component3", "component4"
	verificationOn, verificationOff := v1.VerificationTypeOn, v1.VerificationTypeOff

	// Test
	radixDeployment, err := ApplyDeploymentWithSync(testUtils, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1). // Secret expected
				WithPort("http", 8080).   //
				WithPublicPort("http").   // Public port set
				WithAuthentication(       // Authentication set
					&v1.Authentication{
						ClientCertificate: &v1.ClientCertificate{
							Verification:              &verificationOn,
							PassCertificateToUpstream: pointers.Ptr(true),
						},
					},
				),
			utils.NewDeployComponentBuilder().
				WithName(componentName2). // Secret not expected; No Secret needed based on Certificate
				WithPort("http", 8081).   //
				WithPublicPort("http").   // Public port set
				WithAuthentication(       // Authentication set
					&v1.Authentication{
						ClientCertificate: &v1.ClientCertificate{
							Verification:              &verificationOff,
							PassCertificateToUpstream: pointers.Ptr(false),
						},
					},
				),
			utils.NewDeployComponentBuilder().
				WithName(componentName3). // Secret not expected; No "Public port" set
				WithPort("http", 8082).   //
				WithAuthentication(       // Authentication set
					&v1.Authentication{
						ClientCertificate: &v1.ClientCertificate{
							Verification:              &verificationOn,
							PassCertificateToUpstream: pointers.Ptr(true),
						},
					},
				),
			utils.NewDeployComponentBuilder().
				WithName(componentName4). // Secret not expected; No "Authentication" supplied
				WithPort("https", 8443).  //
				WithPublicPort("https"),  // Public port set
		),
	)

	assert.NoError(t, err)
	assert.NotNil(t, radixDeployment)

	t.Run("validate secrets", func(t *testing.T) {
		var secretName string
		var secretExists bool

		envNamespace := utils.GetEnvironmentNamespace(appName, environment)
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")

		secretName = utils.GetComponentClientCertificateSecretName(componentName1)
		secretExists = secretByNameExists(secretName, secrets)
		assert.True(t, secretExists, "expected secret to exist")

		secretName = utils.GetComponentClientCertificateSecretName(componentName2)
		secretExists = secretByNameExists(secretName, secrets)
		assert.False(t, secretExists, "expected secret not to exist")

		secretName = utils.GetComponentClientCertificateSecretName(componentName3)
		secretExists = secretByNameExists(secretName, secrets)
		assert.False(t, secretExists, "expected secret not to exist")

		secretName = utils.GetComponentClientCertificateSecretName(componentName4)
		secretExists = secretByNameExists(secretName, secrets)
		assert.False(t, secretExists, "expected secret not to exist")
	})

	teardownSecretsTest()
}
