package environments

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	"github.com/equinor/radix-operator/api-server/api/secrets"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type secretsTestSuite struct {
	suite.Suite
	commonTestUtils      *commontest.Utils
	envController        *controllertest.TestUtils
	secretController     *controllertest.TestUtils
	kubeClient           *kubefake.Clientset
	radixClient          *radixfake.Clientset
	kedaClient           *kedafake.Clientset
	dynamicClient        client.Client
	secretProviderClient *secretproviderfake.Clientset
	certClient           *certclientfake.Clientset
}

func TestSecretsTestSuite(t *testing.T) {
	suite.Run(t, new(secretsTestSuite))
}

func (s *secretsTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewClientset()
	s.radixClient = radixfake.NewSimpleClientset() //nolint:staticcheck
	s.kedaClient = kedafake.NewSimpleClientset()
	s.dynamicClient = commontest.CreateClient()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.certClient = certclientfake.NewSimpleClientset()

	commonTestUtils := commontest.NewTestUtils(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	s.commonTestUtils = &commonTestUtils
	s.Require().NoError(s.commonTestUtils.CreateClusterPrerequisites())

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(s.T()))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	envController := controllertest.NewTestUtils(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient, s.certClient, nil, mockValidator, NewEnvironmentController(NewEnvironmentHandlerFactory()))
	s.envController = &envController
	secretController := controllertest.NewTestUtils(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient, s.certClient, nil, mockValidator, secrets.NewSecretController(nil))
	s.secretController = &secretController
}

func (s *secretsTestSuite) SetupSubTest() {
	s.SetupTest()
}

// Generic secrets

func (s *secretsTestSuite) TestGenericSecrets_InitialState() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		secretName    = "SECRET1"
	)
	s.applyDeployment(appName, envName,
		[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithSecrets([]string{secretName})},
		[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithSecrets([]string{secretName})},
	)

	s.assertSecrets([]secretModels.Secret{
		{Name: secretName, DisplayName: secretName, Type: secretModels.SecretTypeGeneric, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: secretName, DisplayName: secretName, Type: secretModels.SecretTypeGeneric, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
	}, s.getSecrets(appName, envName))
}

func (s *secretsTestSuite) TestGenericSecrets_Update() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		secretName1   = "SECRET1"
		secretName2   = "SECRET2"
	)
	s.applyDeployment(appName, envName,
		[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithSecrets([]string{secretName1, secretName2})},
		[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithSecrets([]string{secretName1, secretName2})},
	)

	updatedAt := time.Now()
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, componentName, secretName1, "new-value"))
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, jobName, secretName2, "new-value"))

	s.assertSecrets([]secretModels.Secret{
		{Name: secretName1, DisplayName: secretName1, Type: secretModels.SecretTypeGeneric, Component: componentName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
		{Name: secretName2, DisplayName: secretName2, Type: secretModels.SecretTypeGeneric, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: secretName1, DisplayName: secretName1, Type: secretModels.SecretTypeGeneric, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: secretName2, DisplayName: secretName2, Type: secretModels.SecretTypeGeneric, Component: jobName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
	}, s.getSecrets(appName, envName))
}

// Azure Blob volume credential secrets

func (s *secretsTestSuite) TestAzureBlobVolumeSecrets_InitialState() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		volumeName    = "volume1"
	)
	tests := map[string]struct {
		blobFuse2       v1.RadixBlobFuse2VolumeMount
		expectedSecrets []secretModels.Secret
	}{
		"account key and account name are pending when no value is set": {
			blobFuse2: v1.RadixBlobFuse2VolumeMount{Container: "container"},
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-" + volumeName + "-csiazurecreds-accountname", DisplayName: "Account Name", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountName, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: jobName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: jobName + "-" + volumeName + "-csiazurecreds-accountname", DisplayName: "Account Name", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountName, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
			},
		},
		{
			name: "Azure Blob volumes credential secrets with existing secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Name: "volume1",
							BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
								Container: "container1",
							},
						},
					},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Name: "volume2",
							BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
								Container: "container2",
							},
						},
					},
				},
			},
			existingSecrets: []testSecretDescription{
				{
					secretName: "component1-volume1-csiazurecreds",
					secretData: map[string][]byte{
						"accountname": []byte("current account name1"),
						"accountkey":  []byte("current account key1"),
					},
				},
				{
					secretName: "job1-volume2-csiazurecreds",
					secretData: map[string][]byte{
						"accountname": []byte("current account name2"),
						"accountkey":  []byte("current account key2"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key for storageaccount1", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: jobName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key for storageaccount1", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
			},
		},
		"no secrets when using Azure identity": {
			blobFuse2:       v1.RadixBlobFuse2VolumeMount{Container: "container", UseAzureIdentity: new(true), StorageAccount: "storageaccount1", ResourceGroup: "resource-group1", SubscriptionId: "subscription-id1"},
			expectedSecrets: nil,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			volumeMount := v1.RadixVolumeMount{Name: volumeName, Path: "/path", BlobFuse2: &test.blobFuse2}
			s.applyDeployment(appName, envName,
				[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(volumeMount)},
				[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithVolumeMounts(volumeMount)},
			)

			s.assertSecrets(test.expectedSecrets, s.getSecrets(appName, envName))
		})
	}
}

func (s *secretsTestSuite) TestAzureBlobVolumeSecrets_Update() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		volumeName    = "volume1"
	)
	volumeMount := v1.RadixVolumeMount{Name: volumeName, Path: "/path", BlobFuse2: &v1.RadixBlobFuse2VolumeMount{Container: "container"}}
	s.applyDeployment(appName, envName,
		[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithVolumeMounts(volumeMount)},
		[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithVolumeMounts(volumeMount)},
	)

	updatedAt := time.Now()
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, componentName, componentName+"-"+volumeName+"-csiazurecreds-accountkey", "new-key"))
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, jobName, jobName+"-"+volumeName+"-csiazurecreds-accountname", "new-name"))

	s.assertSecrets([]secretModels.Secret{
		{Name: componentName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: componentName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
		{Name: componentName + "-" + volumeName + "-csiazurecreds-accountname", DisplayName: "Account Name", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountName, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: jobName + "-" + volumeName + "-csiazurecreds-accountkey", DisplayName: "Account Key", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountKey, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: jobName + "-" + volumeName + "-csiazurecreds-accountname", DisplayName: "Account Name", Type: secretModels.SecretTypeCsiAzureBlobVolume, Resource: volumeName, ID: secretModels.SecretIdAccountName, Component: jobName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
	}, s.getSecrets(appName, envName))
}

// Azure Key Vault credential secrets

func (s *secretsTestSuite) TestAzureKeyVaultCredentialSecrets_InitialState() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		keyVaultName  = "keyvault1"
		itemName      = "item1"
	)
	keyVaultItemSecrets := []secretModels.Secret{
		{Name: "AzureKeyVaultItem-" + componentName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: componentName, Status: secretModels.NotAvailable.String(), Updated: nil},
		{Name: "AzureKeyVaultItem-" + jobName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: jobName, Status: secretModels.NotAvailable.String(), Updated: nil},
	}

	tests := map[string]struct {
		useAzureIdentity *bool
		expectedSecrets  []secretModels.Secret
	}{
		"client id and client secret are pending when no value is set": {
			useAzureIdentity: nil,
			expectedSecrets: append([]secretModels.Secret{
				{Name: componentName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientid", DisplayName: "Client ID", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientId, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientSecret, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: jobName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientid", DisplayName: "Client ID", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientId, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: jobName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientSecret, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
			}, keyVaultItemSecrets...),
		},
		"no credential secrets when using Azure identity": {
			useAzureIdentity: new(true),
			expectedSecrets:  keyVaultItemSecrets,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			secretRefs := v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:             keyVaultName,
				UseAzureIdentity: test.useAzureIdentity,
				Items:            []v1.RadixAzureKeyVaultItem{{Name: itemName, EnvVar: "ITEM1"}},
			}}}
			identity := &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any-client-id"}}
			s.applyDeployment(appName, envName,
				[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithSecretRefs(secretRefs).WithIdentity(identity)},
				[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithSecretRefs(secretRefs).WithIdentity(identity)},
			)

			s.assertSecrets(test.expectedSecrets, s.getSecrets(appName, envName))
		})
	}
}

func (s *secretsTestSuite) TestAzureKeyVaultCredentialSecrets_Update() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		keyVaultName  = "keyvault1"
		itemName      = "item1"
	)
	secretRefs := v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{
		Name:  keyVaultName,
		Items: []v1.RadixAzureKeyVaultItem{{Name: itemName, EnvVar: "ITEM1"}},
	}}}
	s.applyDeployment(appName, envName,
		[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithSecretRefs(secretRefs)},
		[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithSecretRefs(secretRefs)},
	)

	updatedAt := time.Now()
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, componentName, componentName+"-"+keyVaultName+"-csiazkvcreds-azkv-clientid", "new-id"))
	s.Require().Equal(http.StatusOK, s.updateSecret(appName, envName, jobName, jobName+"-"+keyVaultName+"-csiazkvcreds-azkv-clientsecret", "new-secret"))

	s.assertSecrets([]secretModels.Secret{
		{Name: componentName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientid", DisplayName: "Client ID", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientId, Component: componentName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
		{Name: componentName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientSecret, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: "AzureKeyVaultItem-" + componentName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: componentName, Status: secretModels.NotAvailable.String(), Updated: nil},
		{Name: jobName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientid", DisplayName: "Client ID", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientId, Component: jobName, Status: secretModels.Pending.String(), Updated: nil},
		{Name: jobName + "-" + keyVaultName + "-csiazkvcreds-azkv-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeCsiAzureKeyVaultCreds, Resource: keyVaultName, ID: secretModels.SecretIdClientSecret, Component: jobName, Status: secretModels.Consistent.String(), Updated: &updatedAt},
		{Name: "AzureKeyVaultItem-" + jobName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: jobName, Status: secretModels.NotAvailable.String(), Updated: nil},
	}, s.getSecrets(appName, envName))
}

// Azure Key Vault item secrets are synced from Azure Key Vault by the CSI driver.
// They cannot be updated through the Radix API, and Updated is never set.

func (s *secretsTestSuite) TestAzureKeyVaultItemSecrets_InitialState() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
		jobName       = "job1"
		keyVaultName  = "keyvault1"
		itemName      = "item1"
	)
	tests := map[string]struct {
		csiDriverHasCreatedSecrets bool
		expectedStatus             string
	}{
		"not available when CSI driver has not created the secret": {
			csiDriverHasCreatedSecrets: false,
			expectedStatus:             secretModels.NotAvailable.String(),
		},
		"consistent when CSI driver has created the secret": {
			csiDriverHasCreatedSecrets: true,
			expectedStatus:             secretModels.Consistent.String(),
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			secretRefs := v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:             keyVaultName,
				UseAzureIdentity: new(true),
				Items:            []v1.RadixAzureKeyVaultItem{{Name: itemName, EnvVar: "ITEM1"}},
			}}}
			identity := &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "any-client-id"}}
			s.applyDeployment(appName, envName,
				[]operatorutils.DeployComponentBuilder{operatorutils.NewDeployComponentBuilder().WithName(componentName).WithSecretRefs(secretRefs).WithIdentity(identity)},
				[]operatorutils.DeployJobComponentBuilder{operatorutils.NewDeployJobComponentBuilder().WithName(jobName).WithSecretRefs(secretRefs).WithIdentity(identity)},
			)
			if test.csiDriverHasCreatedSecrets {
				namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
				secretProviderClasses, err := s.secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).List(context.Background(), metav1.ListOptions{})
				s.Require().NoError(err)
				s.Require().Len(secretProviderClasses.Items, 2)
				for _, secretProviderClass := range secretProviderClasses.Items {
					for _, secretObject := range secretProviderClass.Spec.SecretObjects {
						_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
							Name: secretObject.SecretName, Labels: map[string]string{"secrets-store.csi.k8s.io/managed": "true"},
						}, metav1.CreateOptions{})
						s.Require().NoError(err)
					}
				}
			}

			s.assertSecrets([]secretModels.Secret{
				{Name: "AzureKeyVaultItem-" + componentName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: componentName, Status: test.expectedStatus, Updated: nil},
				{Name: "AzureKeyVaultItem-" + jobName + "--" + keyVaultName + "--secret--" + itemName, DisplayName: "secret " + itemName, Type: secretModels.SecretTypeCsiAzureKeyVaultItem, Resource: keyVaultName, ID: "secret/" + itemName, Component: jobName, Status: test.expectedStatus, Updated: nil},
			}, s.getSecrets(appName, envName))
		})
	}
}

// OAuth2 proxy secrets. The k8s secret is created by the OAuth2 auxiliary resource manager, which
// is not run by the deployment sync in these tests, so tests create it explicitly when needed.

func (s *secretsTestSuite) TestOAuth2Secrets_InitialState() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
	)
	tests := map[string]struct {
		component       operatorutils.DeployComponentBuilder
		expectedSecrets []secretModels.Secret
	}{
		"client secret and cookie secret are pending when no value is set": {
			component: operatorutils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8080).WithPublicPort("http").
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
			},
		},
		"redis password is pending when session store is redis and no value is set": {
			component: operatorutils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8080).WithPublicPort("http").
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-redispassword", DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
			},
		},
		"no client secret when using Azure workload identity": {
			component: operatorutils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8080).WithPublicPort("http").
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{Credentials: v1.AzureWorkloadIdentity}}),
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
			},
		},
		"no secrets when component is not public": {
			component: operatorutils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8080).
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			expectedSecrets: nil,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.applyDeployment(appName, envName, []operatorutils.DeployComponentBuilder{test.component}, nil)

			s.assertSecrets(test.expectedSecrets, s.getSecrets(appName, envName))
		})
	}
}

func (s *secretsTestSuite) TestOAuth2Secrets_Update() {
	const (
		appName       = "any-app"
		envName       = "dev"
		componentName = "comp1"
	)
	tests := map[string]struct {
		oauth2                 v1.OAuth2
		k8sSecretExists        bool
		secretsToUpdate        []string
		expectedResponseStatus int
		expectedSecrets        []secretModels.Secret
	}{
		"updated clientsecret is consistent and have updated timestamp": {
			oauth2:                 v1.OAuth2{SessionStoreType: v1.SessionStoreRedis},
			k8sSecretExists:        true,
			secretsToUpdate:        []string{componentName + "-oauth2proxy-clientsecret"},
			expectedResponseStatus: http.StatusOK,
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: new(time.Now())},
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-redispassword", DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
			},
		},
		"updated cookiesecret is consistent and have updated timestamp": {
			oauth2:                 v1.OAuth2{SessionStoreType: v1.SessionStoreRedis},
			k8sSecretExists:        true,
			secretsToUpdate:        []string{componentName + "-oauth2proxy-cookiesecret"},
			expectedResponseStatus: http.StatusOK,
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: new(time.Now())},
				{Name: componentName + "-oauth2proxy-redispassword", DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
			},
		},
		"updated redispassword is consistent and have updated timestamp": {
			oauth2:                 v1.OAuth2{SessionStoreType: v1.SessionStoreRedis},
			k8sSecretExists:        true,
			secretsToUpdate:        []string{componentName + "-oauth2proxy-redispassword"},
			expectedResponseStatus: http.StatusOK,
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-clientsecret", DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Pending.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
				{Name: componentName + "-oauth2proxy-redispassword", DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: new(time.Now())},
			},
		},
		"bad request when updating client secret while using Azure workload identity": {
			oauth2:                 v1.OAuth2{Credentials: v1.AzureWorkloadIdentity},
			k8sSecretExists:        true,
			secretsToUpdate:        []string{componentName + "-oauth2proxy-clientsecret"},
			expectedResponseStatus: http.StatusBadRequest,
			expectedSecrets: []secretModels.Secret{
				{Name: componentName + "-oauth2proxy-cookiesecret", DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: componentName, Status: secretModels.Consistent.String(), Updated: nil},
			},
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.applyDeployment(appName, envName, []operatorutils.DeployComponentBuilder{
				operatorutils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8080).WithPublicPort("http").
					WithAuthentication(&v1.Authentication{OAuth2: &test.oauth2}),
			}, nil)

			for _, secretName := range test.secretsToUpdate {
				s.Equal(test.expectedResponseStatus, s.updateSecret(appName, envName, componentName, secretName, "new-value"))
			}

			s.assertSecrets(test.expectedSecrets, s.getSecrets(appName, envName))
		})
	}
}

func (s *secretsTestSuite) applyDeployment(appName, envName string, components []operatorutils.DeployComponentBuilder, jobs []operatorutils.DeployJobComponentBuilder) {
	deployment := operatorutils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithRadixApplication(operatorutils.NewRadixApplicationBuilder().
			WithRadixRegistration(operatorutils.ARadixRegistration()).
			WithAppName(appName).
			WithEnvironment(envName, "master")).
		WithComponents(components...).
		WithJobComponents(jobs...)
	err := controllertest.ApplyDeploymentWithSync(s.kubeClient, s.radixClient, s.kedaClient, s.dynamicClient, s.commonTestUtils, s.secretProviderClient, s.certClient, deployment)
	s.Require().NoError(err)
}

func (s *secretsTestSuite) getSecrets(appName, envName string) []secretModels.Secret {
	response := <-s.envController.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, envName))
	s.Require().Equal(http.StatusOK, response.Code)
	var environment environmentModels.Environment
	s.Require().NoError(controllertest.GetResponseBody(response, &environment))
	return environment.Secrets
}

func (s *secretsTestSuite) updateSecret(appName, envName, componentName, secretName, value string) int {
	response := <-s.secretController.ExecuteRequestWithParameters(http.MethodPut,
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", appName, envName, componentName, secretName),
		secretModels.SecretParameters{SecretValue: value})
	return response.Code
}

// assertSecrets requires actual to have the same number of secrets as expected, looks up each expected secret
// in actual by name and component, and compares every field. Updated must be within 5 seconds of the expected time.
func (s *secretsTestSuite) assertSecrets(expected, actual []secretModels.Secret) {
	s.T().Helper()
	s.Require().Len(actual, len(expected))
	type secretKey struct{ name, component string }
	actualByKey := make(map[secretKey]secretModels.Secret, len(actual))
	for _, secret := range actual {
		actualByKey[secretKey{secret.Name, secret.Component}] = secret
	}
	for _, expectedSecret := range expected {
		name, component := expectedSecret.Name, expectedSecret.Component
		actualSecret, found := actualByKey[secretKey{name, component}]
		if !s.True(found, "secret %s in %s not found", name, component) {
			continue
		}
		s.Equal(expectedSecret.DisplayName, actualSecret.DisplayName, "DisplayName of secret %s in %s", name, component)
		s.Equal(expectedSecret.Type, actualSecret.Type, "Type of secret %s in %s", name, component)
		s.Equal(expectedSecret.Resource, actualSecret.Resource, "Resource of secret %s in %s", name, component)
		s.Equal(expectedSecret.ID, actualSecret.ID, "ID of secret %s in %s", name, component)
		s.Equal(expectedSecret.Status, actualSecret.Status, "Status of secret %s in %s", name, component)
		if expectedSecret.Updated == nil {
			s.Nil(actualSecret.Updated, "Updated of secret %s in %s", name, component)
		} else if s.NotNil(actualSecret.Updated, "Updated of secret %s in %s", name, component) {
			s.WithinDuration(*expectedSecret.Updated, *actualSecret.Updated, 5*time.Second, "Updated of secret %s in %s", name, component)
		}
	}
}
