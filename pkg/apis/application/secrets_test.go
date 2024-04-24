package application

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	_ "github.com/equinor/radix-operator/pkg/apis/test/initlogger"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

const somePrivateKey = "-----BEGIN RSA PRIVATE KEY-----\n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\n-----END RSA PRIVATE KEY-----"
const somePublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDpMmE/GGn1U86URGIWyfVOOVHlqI+Q/9MF5EkvuYEslbdJgaHHkolC86hIHGO++RXqyOMudMyWFVTVSTMDDdo+GGrKWjYrmsIFEyPg5mQ6oLRsE6XR/8maKtusvXGw/2jIGp4Dkna7X0HeueH0crZpWkdHO4x/gm9Yb5Sm/wK2UzvsEyWD9gK0t/krtKh4cZtWPo+802gCCqTeeaq5g1jye4Kl/tlstF0tOuM0we3O583R3wIPaOumhfTLEMztKBzYkAA0PddWA8TPuolCshmIzZzScRN8sQzrXB4rHd6Hs0i85+7I1xJzHusZMwCCyO9hBzLI4F5p4+rpHjuV8MJX3dy7J24yebSvot9UVNWvGANP6C2PsTQitDBQ6bpaWd1ym6O66SQ73EJQphcoeNFGCv3vDEhnSM8nKkg1ivUPpzYZZNLgCFZmWJZtyKMEoZFgq1k8PcyIxfeQ/rQFAeZ1+Rh2aP1+iIdZ5MPxDWHCl3N/ObgrgIqYh/roO70TY20="

func TestOnSync_PublicKeyCmExists_NothingChanges(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// check secret does exist
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, map[string]string(labels.ForApplicationName(appName)), secret.Labels)

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

func TestOnSync_PublicKeyCmDoesNotExist_NewKeyIsGenerated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName)

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
	tu, client, kubeUtil, radixClient := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithPublicKey(somePublicKey).
		WithPrivateKey(somePrivateKey)

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
	assert.Equal(t, somePublicKey, publicKey)

	// check secret exists, and has same private key as RR
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.Equal(t, []byte(somePrivateKey), privateKey)
}

func TestOnSync_PublicKeyInCmIsEmpty_KeyIsCopiedFromRR(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithPublicKey(somePublicKey).
		WithPrivateKey(somePrivateKey)

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
	assert.Equal(t, somePublicKey, strings.TrimSpace(publicKey))

	// check secret exists, and has same private key as RR
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	privateKey := secret.Data[defaults.GitPrivateKeySecretKey]
	assert.Equal(t, []byte(somePrivateKey), privateKey)

}

func TestOnSync_PrivateKeySecretIsUpdatedManually_PublicKeyIsUpdated(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixClient := setupTest(t)
	defer os.Clearenv()

	// Test
	appName := "any-app"
	rr := utils.ARadixRegistration().
		WithName(appName).
		WithPublicKey(somePublicKey).
		WithPrivateKey(somePrivateKey)

	_, err := applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)

	// modify the private key secret manually
	secret, err := client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)

	deployKey, err := utils.GenerateDeployKey()
	assert.NoError(t, err)

	newSecret := secret.DeepCopy()
	newSecret.Data[defaults.GitPrivateKeySecretKey] = []byte(deployKey.PrivateKey)

	_, err = client.CoreV1().Secrets(utils.GetAppNamespace(appName)).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
	assert.NoError(t, err)
	_, err = applyRegistrationWithSync(tu, client, kubeUtil, radixClient, rr)
	assert.NoError(t, err)
	// check that the public key cm is updated
	cm, err := client.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Get(context.TODO(), defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	publicKey := cm.Data[defaults.GitPublicKeyConfigMapKey]
	assert.Equal(t, deployKey.PublicKey, publicKey)

}
