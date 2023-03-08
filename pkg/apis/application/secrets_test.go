package application

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func TestOnSync_PublicKeyCmExists_NothingChanges(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check secret does exist
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NotNil(t, secret)

	// check public key cm exists
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NotNil(t, cm)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check public key cm still exists and has same key
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NotNil(t, cm)
	newPublicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, publicKey, newPublicKey)

	// check secret still exists and has same key
	newSecret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.Equal(t, secret, newSecret)

}

func TestOnSync_PublicKeyCmExists_OwnerReferenceIsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// TODO: check public key cm exists, and remove ownerReference

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// TODO: check public key cm still exists and has same key, but now has ownerReference
}

func TestOnSync_PublicKeyCmDoesNotExist_NewKeyIsGenerated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true)

	// TODO: check public key cm does not exist
	// TODO: check secret does not exist

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// TODO: check public key cm exists, and has key
	// TODO: check secret exists, and has private key

}

func TestOnSync_PublicKeyCmDoesNotExist_KeyIsCopiedFromRR(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true).
		WithPublicKey("somepublicKey").
		WithPrivateKey("someprivateKey")

	// TODO: check public key cm does not exist

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// TODO: check public key cm exists, and has same public key as RR
	// TODO: check secret exists, and has same private key as RR

}