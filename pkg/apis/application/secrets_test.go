package application

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func TestOnSync_PublicKeyCmExists_NothingChanges(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check secret does exist
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, map[string]string(labels.ForApplicationName(appName)), secret.Labels)

	// check public key cm exists
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)
	assert.NoError(t, err)

	// check public key cm still exists and has same key
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	newPublicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, publicKey, newPublicKey)

	// check secret still exists and has same key
	newSecret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, secret, newSecret)

}

func TestOnSync_PublicKeyCmDoesNotExist_NewKeyIsGenerated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

	// check public key cm does not exist
	_, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.Error(t, err)
	// check secret does not exist
	_, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.Error(t, err)

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check public key cm exists, and has key
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.NotNil(t, publicKey)

	// check secret exists, and has private key
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.NotNil(t, privateKey)

}

func TestOnSync_WebhookSharedSecret_GeneratedAndNotOverwritten(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// The operator creates and seeds the shared secret in the app namespace.
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Data[defaults.WebhookSharedSecretKey])
	assert.Equal(t, map[string]string(labels.ForApplicationName(appName)), secret.Labels)

	// Simulate a regeneration performed through the api-server directly on the secret (base64-encoded value)
	secret.Data[defaults.WebhookSharedSecretKey] = []byte("regenerated-secret")
	_, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Update(context.Background(), secret, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// A subsequent sync must not overwrite the existing valid secret
	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	secret, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "regenerated-secret", string(secret.Data[defaults.WebhookSharedSecretKey]))
}

func TestOnSync_WebhookSharedSecret_GeneratedWhenSpecEmpty(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Data[defaults.WebhookSharedSecretKey])
}

func TestOnSync_WebhookSharedSecret_MissingSecretIsCreatedFromSpecSharedSecret(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	appName := "any-app"
	expectedSharedSecret := "shared-secret-from-spec"
	rr := utils.ARadixRegistration().WithName(appName)

	registration, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// Remove existing secret to simulate missing secret in app namespace.
	err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Delete(context.Background(), defaults.WebhookSharedSecretName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	// Set deprecated field that should seed the recreated secret.
	registration.Spec.SharedSecret = expectedSharedSecret //nolint:staticcheck
	registration, err = radixClient.RadixV1().RadixRegistrations().Update(context.Background(), registration, metav1.UpdateOptions{})
	assert.NoError(t, err)

	application := NewApplication(client, kubeUtil, radixClient, registration)
	err = application.OnSync(context.Background())
	assert.NoError(t, err)

	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedSharedSecret, string(secret.Data[defaults.WebhookSharedSecretKey]))
}

func TestOnSync_WebhookSharedSecret_MissingKeyIsRepaired(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient, _ := setupTest(t)
	defer os.Clearenv()

	appName := "any-app"
	rr := utils.ARadixRegistration().WithName(appName)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// Empty the secret data
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	secret.Data = map[string][]byte{}
	_, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Update(context.Background(), secret, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// A subsequent sync repairs the missing data by generating a new valid shared secret.
	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	secret, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.Background(), defaults.WebhookSharedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Data[defaults.WebhookSharedSecretKey])
}
