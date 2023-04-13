package application

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strings"
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
	assert.NoError(t, err)
	assert.NotNil(t, secret)

	// check public key cm exists
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)
	assert.NoError(t, err)

	// check public key cm still exists and has same key
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	newPublicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, publicKey, newPublicKey)

	// check secret still exists and has same key
	newSecret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, secret, newSecret)

}

func TestOnSync_PublicKeyCmExists_OwnerReferenceIsSet(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rrBuilder := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rrBuilder)
	assert.NoError(t, err)

	// check public key cm exists, and remove ownerReference
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	desiredCm := cm.DeepCopy()
	desiredCm.OwnerReferences = nil

	err = kubeUtil.ApplyConfigMap(utils.GetAppNamespace(appName), cm, desiredCm)
	assert.NoError(t, err)

	// check secret exists, and remove ownerReference
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	secret.OwnerReferences = nil

	_, err = kubeUtil.ApplySecret(utils.GetAppNamespace(appName), secret)
	assert.NoError(t, err)

	rr, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rrBuilder)
	assert.NoError(t, err)

	// check secret still exists, but now has ownerReference
	secret, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, secret.OwnerReferences, GetOwnerReferenceOfRegistration(rr))

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

	// check public key cm does not exist
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.Nil(t, cm)
	// check secret does not exist
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.Nil(t, secret)

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)
	assert.NoError(t, err)

	// check public key cm exists, and has key
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.NotNil(t, publicKey)

	// check secret exists, and has private key
	secret, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NoError(t, err)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.NotNil(t, privateKey)

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
		WithPublicKey(test.PublicKey).
		WithPrivateKey(test.PrivateKey)

	// check public key cm does not exist
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.Nil(t, cm)

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check public key cm exists, and has same public key as RR
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, test.PublicKey, publicKey)

	// check secret exists, and has same private key as RR
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.Equal(t, privateKey, []byte(test.PrivateKey))
}

func TestOnSync_PublicKeyInCmIsEmpty_KeyIsCopiedFromRR(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithMachineUser(true).
		WithPublicKey(test.PublicKey).
		WithPrivateKey(test.PrivateKey)

	// check public key cm does not exist
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.Nil(t, cm)

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// delete data in public key cm
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	cm.Data = map[string]string{}
	_, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Update(context.TODO(), cm, metav1.UpdateOptions{})
	assert.NoError(t, err)

	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check public key cm exists, and has same public key as RR
	cm, err = client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, test.PublicKey, strings.TrimSpace(publicKey))

	// check secret exists, and has same private key as RR
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.Equal(t, []byte(test.PrivateKey), privateKey)

}
