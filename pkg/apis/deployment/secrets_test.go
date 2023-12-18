package deployment

import (
	"context"
	"os"
	"testing"

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

func setupSecretsTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, prometheusclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	handlerTestUtils.CreateClusterPrerequisites(testClusterName, testEgressIps, "anysubid")
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient, prometheusclient
}

func teardownSecretsTest() {
	// Cleanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	os.Unsetenv(defaults.OperatorClusterTypeEnvironmentVariable)
}

func TestSecretDeployed_ClientCertificateSecretGetsSet(t *testing.T) {
	// Setup
	testUtils, client, kubeUtil, radixclient, prometheusclient := setupSecretsTest()
	appName, environment := "edcradix", "test"
	componentName1, componentName2, componentName3, componentName4 := "component1", "component2", "component3", "component4"
	verificationOn, verificationOff := v1.VerificationTypeOn, v1.VerificationTypeOff

	// Test
	radixDeployment, err := applyDeploymentWithSync(testUtils, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
							PassCertificateToUpstream: utils.BoolPtr(true),
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
							PassCertificateToUpstream: utils.BoolPtr(false),
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
							PassCertificateToUpstream: utils.BoolPtr(true),
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
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
