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
