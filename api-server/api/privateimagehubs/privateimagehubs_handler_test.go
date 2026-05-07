package privateimagehubs_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/privateimagehubs/internal"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName = "AnyClusterName"
)

func Test_WithPrivateImageHubSet_SecretsCorrectly_NoImageHubs(t *testing.T) {
	kubeUtil, err := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{})
	require.NoError(t, err)
	secret, err := kubeUtil.GetSecret(context.TODO(), "any-app-app", defaults.PrivateImageHubSecretName)
	require.NoError(t, err)
	pendingSecrets, _ := internal.GetPendingPrivateImageHubSecrets(secret)

	assert.NotNil(t, secret)
	assert.Equal(t,
		"{\"auths\":{}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, 0, len(pendingSecrets))
	assert.Error(t, internal.UpdatePrivateImageHubsSecretsPassword(context.Background(), kubeUtil, "any-app", "privaterepodeleteme.azurecr.io", "a-password"))
}

func Test_WithPrivateImageHubSet_SecretsCorrectly_SetPassword(t *testing.T) {
	kubeUtil, err := applyRadixAppWithPrivateImageHub(radixv1.PrivateImageHubEntries{
		"privaterepodeleteme.azurecr.io": &radixv1.RadixPrivateImageHubCredential{
			Username: "814607e6-3d71-44a7-8476-50e8b281abbc",
			Email:    "radix@equinor.com",
		},
	})
	require.NoError(t, err)

	secret, err := kubeUtil.GetSecret(context.Background(), "any-app-app", defaults.PrivateImageHubSecretName)
	require.NoError(t, err)
	pendingSecrets, _ := internal.GetPendingPrivateImageHubSecrets(secret)

	assert.Equal(t, "privaterepodeleteme.azurecr.io", pendingSecrets[0])

	if err := internal.UpdatePrivateImageHubsSecretsPassword(context.Background(), kubeUtil, "any-app", "privaterepodeleteme.azurecr.io", "a-password"); err != nil {
		require.NoError(t, err)
	}
	secret, err = kubeUtil.GetSecret(context.Background(), "any-app-app", defaults.PrivateImageHubSecretName)
	require.NoError(t, err)
	pendingSecrets, _ = internal.GetPendingPrivateImageHubSecrets(secret)

	assert.Equal(t,
		"{\"auths\":{\"privaterepodeleteme.azurecr.io\":{\"username\":\"814607e6-3d71-44a7-8476-50e8b281abbc\",\"password\":\"a-password\",\"email\":\"radix@equinor.com\",\"auth\":\"ODE0NjA3ZTYtM2Q3MS00NGE3LTg0NzYtNTBlOGIyODFhYmJjOmEtcGFzc3dvcmQ=\"}}}",
		string(secret.Data[corev1.DockerConfigJsonKey]))
	assert.Equal(t, 0, len(pendingSecrets))
}

func applyRadixAppWithPrivateImageHub(privateImageHubs radixv1.PrivateImageHubEntries) (*kube.Kube, error) {
	tu, client, kubeUtil, radixClient, err := setupTest()
	if err != nil {
		return nil, err
	}
	appBuilder := utils.ARadixApplication().
		WithAppName("any-app").
		WithEnvironment("dev", "master")
	for key, config := range privateImageHubs {
		appBuilder.WithPrivateImageRegistry(key, config.Username, config.Email)
	}

	if err = applyApplicationWithSync(tu, client, kubeUtil, radixClient, appBuilder); err != nil {
		return nil, err
	}
	return kubeUtil, nil
}

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, error) {
	kubeClient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeClient, radixClient, kedaClient, secretproviderclient)
	if err := handlerTestUtils.CreateClusterPrerequisites(clusterName, "anysubid"); err != nil {
		return nil, nil, nil, nil, err
	}
	return &handlerTestUtils, kubeClient, kubeUtil, radixClient, nil
}

func applyApplicationWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixClient radixclient.Interface, applicationBuilder utils.ApplicationBuilder) error {

	ra, err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationConfig := applicationconfig.NewApplicationConfig(client, kubeUtil, radixClient, radixRegistration, ra)

	err = applicationConfig.OnSync(context.Background())
	if err != nil {
		return err
	}

	return nil
}
