package environments

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/api/secrets/suffix"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	secretsstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

type secretHandlerTestSuite struct {
	suite.Suite
}

func TestRunSecretHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(secretHandlerTestSuite))
}

type testSecretDescription struct {
	secretName string
	secretData map[string][]byte
	labels     map[string]string
}

type getSecretScenario struct {
	name                   string
	components             []v1.RadixDeployComponent
	jobs                   []v1.RadixDeployJobComponent
	init                   func(kubernetes.Interface, radixclient.Interface, secretsstoreclient.Interface) // scenario optional custom init function
	existingSecrets        []testSecretDescription
	expectedSecrets        []secretModels.Secret
	expectedSecretVersions map[string]map[string]map[string]map[string]map[string]bool // map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
}

type testSecretProviderClassAndSecret struct {
	secretName string
	className  string
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetSecrets() {
	componentName1 := "component1"
	jobName1 := "job1"
	scenarios := []getSecretScenario{
		{
			name: "regular secrets with no existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_J1",
				},
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "SECRET_C1",
					DisplayName: "SECRET_C1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "regular secrets with existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_J1",
				},
			}},
			existingSecrets: []testSecretDescription{
				{
					secretName: operatorutils.GetComponentSecretName(componentName1),
					secretData: map[string][]byte{"SECRET_C1": []byte("current-value1")},
				},
				{
					secretName: operatorutils.GetComponentSecretName(jobName1),
					secretData: map[string][]byte{"SECRET_J1": []byte("current-value2")},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "SECRET_C1",
					DisplayName: "SECRET_C1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
				},
				{
					Name:        "SECRET_J1",
					DisplayName: "SECRET_J1",
					Type:        secretModels.SecretTypeGeneric,
					Resource:    "",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
				},
			},
		},
		{
			name: "Azure Blob volumes credential secrets with no secrets",
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
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-volume1-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountName,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountName,
				},
			},
		},
		{
			name: "Azure BlobFuse2 volumes credential secrets with predefined StorageAccount",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Name: "volume1",
							BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
								Container:      "container1",
								StorageAccount: "storageaccount1",
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
								Container:      "container2",
								StorageAccount: "storageaccount1",
							},
						},
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-volume1-csiazurecreds-accountkey",
					DisplayName: "Account Key for storageaccount1",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key for storageaccount1",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
			},
		},
		{
			name: "Azure Blob volumes doe not use credential secrets when UseAzureIdentity is set to true",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Name: "volume1",
							BlobFuse2: &v1.RadixBlobFuse2VolumeMount{
								Container:        "container1",
								UseAzureIdentity: pointers.Ptr(true),
								StorageAccount:   "storageaccount1",
								ResourceGroup:    "resource-group1",
								SubscriptionId:   "subscription-id1",
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
								Container:        "container2",
								UseAzureIdentity: pointers.Ptr(true),
								StorageAccount:   "storageaccount1",
								ResourceGroup:    "resource-group1",
								SubscriptionId:   "subscription-id1",
							},
						},
					},
				},
			},
			expectedSecrets: []secretModels.Secret{},
		},
		{
			name: "Azure Blob volumes credential secrets with existing secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobFuse2FuseCsiAzure,
							Name:    "volume1",
							Storage: "container1",
						},
					},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					VolumeMounts: []v1.RadixVolumeMount{
						{
							Type:    v1.MountTypeBlobFuse2FuseCsiAzure,
							Name:    "volume2",
							Storage: "container2",
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
				{
					Name:        "component1-volume1-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "component1-volume1-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume1",
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountName,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountkey",
					DisplayName: "Account Key",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountKey,
				},
				{
					Name:        "job1-volume2-csiazurecreds-accountname",
					DisplayName: "Account Name",
					Type:        secretModels.SecretTypeCsiAzureBlobVolume,
					Resource:    "volume2",
					Component:   jobName1,
					Status:      secretModels.Consistent.String(),
					ID:          secretModels.SecretIdAccountName,
				},
			},
		},
		{
			name: "No Azure Key vault credential secrets when there is no Azure key vault SecretRefs",
			components: []v1.RadixDeployComponent{
				{
					Name:       componentName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: nil},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name:       jobName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: nil},
				},
			},
			expectedSecrets: nil,
		},
		{
			name: "Azure Key vault credential secrets when there are secret items with no secrets",
			components: []v1.RadixDeployComponent{
				{
					Name: componentName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: "keyVault1",
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret1",
									EnvVar: "SECRET_REF1",
								},
							}},
					}},
				},
			},
			jobs: []v1.RadixDeployJobComponent{
				{
					Name: jobName1,
					SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
						{
							Name: "keyVault2",
							Items: []v1.RadixAzureKeyVaultItem{
								{
									Name:   "secret2",
									EnvVar: "SECRET_REF2",
								},
							}},
					}},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-keyvault1-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientId,
				},
				{
					Name:        "component1-keyvault1-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientSecret,
				},
				{
					Name:        "AzureKeyVaultItem-component1--keyVault1--secret--secret1",
					DisplayName: "secret secret1",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault1",
					Component:   componentName1,
					Status:      secretModels.NotAvailable.String(),
					ID:          "secret/secret1",
				},
				{
					Name:        "job1-keyvault2-csiazkvcreds-azkv-clientid",
					DisplayName: "Client ID",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientId,
				},
				{
					Name:        "job1-keyvault2-csiazkvcreds-azkv-clientsecret",
					DisplayName: "Client Secret",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.Pending.String(),
					ID:          secretModels.SecretIdClientSecret,
				},
				{
					Name:        "AzureKeyVaultItem-job1--keyVault2--secret--secret2",
					DisplayName: "secret secret2",
					Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
					Resource:    "keyVault2",
					Component:   jobName1,
					Status:      secretModels.NotAvailable.String(),
					ID:          "secret/secret1",
				},
			},
		},
		{
			name: "Secrets from Authentication with PassCertificateToUpstream with no secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with PassCertificateToUpstream with existing secrets",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			existingSecrets: []testSecretDescription{
				{
					secretName: "component1-clientcertca",
					secretData: map[string][]byte{
						"ca.crt": []byte("current certificate"),
					},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Consistent.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Authentication: &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOn)},
				},
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8000}},
				PublicPort: "http",
			}},
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets for components with OAuth2",
			components: []v1.RadixDeployComponent{
				{
					Name:           "comp1",
					PublicPort:     "http",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{}},
				},
				{
					Name:           "comp2",
					PublicPort:     "http",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}},
				},
				{
					Name:           "comp3",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{}},
				},
				{
					Name:       "comp4",
					PublicPort: "http",
				},
				{
					Name:           "comp5",
					PublicPort:     "http",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{}},
				},
				{
					Name:           "comp6",
					PublicPort:     "http",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}},
				},
				{
					Name:           "comp7",
					PublicPort:     "http",
					Authentication: &v1.Authentication{OAuth2: &v1.OAuth2{Credentials: v1.AzureWorkloadIdentity}},
				},
			},
			expectedSecrets: []secretModels.Secret{
				{Name: "comp1" + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp1", Status: secretModels.Pending.String()},
				{Name: "comp1" + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp1", Status: secretModels.Pending.String()},
				{Name: "comp2" + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp2", Status: secretModels.Pending.String()},
				{Name: "comp2" + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp2", Status: secretModels.Pending.String()},
				{Name: "comp2" + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp2", Status: secretModels.Pending.String()},
				{Name: "comp5" + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp5", Status: secretModels.Consistent.String()},
				{Name: "comp5" + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp5", Status: secretModels.Pending.String()},
				{Name: "comp6" + suffix.OAuth2ClientSecret, DisplayName: "Client Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp6", Status: secretModels.Pending.String()},
				{Name: "comp6" + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp6", Status: secretModels.Consistent.String()},
				{Name: "comp6" + suffix.OAuth2RedisPassword, DisplayName: "Redis Password", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp6", Status: secretModels.Consistent.String()},
				{Name: "comp7" + suffix.OAuth2CookieSecret, DisplayName: "Cookie Secret", Type: secretModels.SecretTypeOAuth2Proxy, Component: "comp7", Status: secretModels.Pending.String()},
			},
			existingSecrets: []testSecretDescription{
				{
					secretName: operatorutils.GetAuxiliaryComponentSecretName("comp5", v1.OAuthProxyAuxiliaryComponentSuffix),
					secretData: map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("any data")},
				},
				{
					secretName: operatorutils.GetAuxiliaryComponentSecretName("comp6", v1.OAuthProxyAuxiliaryComponentSuffix),
					secretData: map[string][]byte{operatordefaults.OAuthCookieSecretKeyName: []byte("any data"), operatordefaults.OAuthRedisPasswordKeyName: []byte("any data")},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := "deployment1"

		s.Run(fmt.Sprintf("test GetSecretsForDeployment: %s", scenario.name), func() {
			controllerUtils := s.prepareTestRun(&scenario, appName, environment, deploymentName)
			responseChannel := controllerUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, environment))
			response := <-responseChannel
			s.LessOrEqual(response.Code, 299)
			var actual environmentModels.Environment
			err := controllertest.GetResponseBody(response, &actual)
			s.Require().Nil(err)
			s.assertSecrets(&scenario, actual.Secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAzureKeyVaultSecretRefVersionStatuses() {
	const deployment1 = "deployment1"
	const componentName1 = "component1"
	const jobName1 = "job1"

	scenarios := []getSecretScenario{
		testCreateScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Not available, when no secret provider class",
			func(scenario *getSecretScenario) {
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, "secret/secret1", secretModels.NotAvailable)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, "secret/secret2", secretModels.NotAvailable)
			}),
		testCreateScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Not available, when exists secret provider class, but no secret",
			func(scenario *getSecretScenario) {
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, "secret/secret1", secretModels.NotAvailable)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, "secret/secret2", secretModels.NotAvailable)
				initFunc := func(_ kubernetes.Interface, _ radixclient.Interface, secretClient secretsstoreclient.Interface) {
					testCreateSecretProviderClass(secretClient, deployment1, &scenario.components[0])
					testCreateSecretProviderClass(secretClient, deployment1, &scenario.jobs[0])
				}
				scenario.init = initFunc
			}),
		testCreateScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(
			"Consistent, when exists secret provider class and secret",
			func(scenario *getSecretScenario) {
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(componentName1, "secret/secret1", secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientId, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, secretModels.SecretIdClientSecret, secretModels.Consistent)
				scenario.testSetExpectedSecretStatus(jobName1, "secret/secret2", secretModels.Consistent)
				initFunc := func(kubeClient kubernetes.Interface, _ radixclient.Interface, secretClient secretsstoreclient.Interface) {
					componentSecretMap := testCreateSecretProviderClass(secretClient, deployment1, &scenario.components[0])
					jobSecretMap := testCreateSecretProviderClass(secretClient, deployment1, &scenario.jobs[0])
					for _, secretProviderClassAndSecret := range componentSecretMap {
						testCreateAzureKeyVaultCsiDriverSecret(kubeClient, secretProviderClassAndSecret.secretName, map[string]string{"SECRET1": "val1"})
					}
					for _, secretProviderClassAndSecret := range jobSecretMap {
						testCreateAzureKeyVaultCsiDriverSecret(kubeClient, secretProviderClassAndSecret.secretName, map[string]string{"SECRET2": "val2"})
					}
				}
				scenario.init = initFunc
			}),
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := deployment1

		s.Run(fmt.Sprintf("test GetSecretsStatus: %s", scenario.name), func() {
			controllerUtils := s.prepareTestRun(&scenario, appName, environment, deploymentName)
			responseChannel := controllerUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, environment))
			response := <-responseChannel
			s.LessOrEqual(response.Code, 299)
			var actual environmentModels.Environment
			err := controllertest.GetResponseBody(response, &actual)
			s.Require().Nil(err)
			s.assertSecrets(&scenario, actual.Secrets)
		})
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAuthenticationSecrets() {
	componentName1 := "component1"
	scenarios := []struct {
		name            string
		modifyComponent func(*v1.RadixDeployComponent)
		expectedError   bool
		expectedSecrets []secretModels.Secret
	}{
		{
			name: "Secrets from Authentication with PassCertificateToUpstream",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{PassCertificateToUpstream: utils.BoolPtr(true)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOn)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOn",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOn)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOptional",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOptional)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "Secrets from Authentication with VerificationTypeOptionalNoCa",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOptionalNoCa)},
				}
				component.PublicPort = "http"
			},
			expectedError: false,
			expectedSecrets: []secretModels.Secret{
				{
					Name:        "component1-clientcertca",
					DisplayName: "",
					Type:        secretModels.SecretTypeClientCertificateAuth,
					Component:   componentName1,
					Status:      secretModels.Pending.String(),
				},
			},
		},
		{
			name: "No secrets from Authentication with VerificationTypeOff",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOff)},
				}
				component.PublicPort = "http"
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
		{
			name: "No secrets from Authentication for not public port",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{Verification: pointers.Ptr(v1.VerificationTypeOn)},
				}
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
		{
			name: "No secrets from Authentication with No Verification and PassCertificateToUpstream",
			modifyComponent: func(component *v1.RadixDeployComponent) {
				component.Authentication = &v1.Authentication{
					ClientCertificate: &v1.ClientCertificate{},
				}
				component.PublicPort = "http"
			},
			expectedError:   false,
			expectedSecrets: []secretModels.Secret{},
		},
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			environment := anyEnvironment
			appName := anyAppName
			deploymentName := "deployment1"
			commonScenario := getSecretScenario{
				name: scenario.name,
				components: []v1.RadixDeployComponent{{
					Name:  componentName1,
					Ports: []v1.ComponentPort{{Name: "http", Port: 8000}},
				}},
				expectedSecrets: scenario.expectedSecrets,
			}
			scenario.modifyComponent(&commonScenario.components[0])

			controllerUtils := s.prepareTestRun(&commonScenario, appName, environment, deploymentName)
			responseChannel := controllerUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, environment))
			response := <-responseChannel
			s.LessOrEqual(response.Code, 299)
			var actual environmentModels.Environment
			err := controllertest.GetResponseBody(response, &actual)
			s.Require().Nil(err)
			s.assertSecrets(&commonScenario, actual.Secrets)
		})
	}
}

func (s *secretHandlerTestSuite) assertSecrets(scenario *getSecretScenario, secrets []secretModels.Secret) {
	s.Equal(len(scenario.expectedSecrets), len(secrets))
	secretMap := testGetSecretMap(secrets)
	for _, expectedSecret := range scenario.expectedSecrets {
		secret, exists := secretMap[expectedSecret.Name]
		s.True(exists, "Missed secret %s", expectedSecret.Name)
		s.Equal(expectedSecret.Type, secret.Type, "Not expected secret Type for %s", expectedSecret.String())
		s.Equal(expectedSecret.Component, secret.Component, "Not expected secret Component for %s", expectedSecret.String())
		s.Equal(expectedSecret.DisplayName, secret.DisplayName, "Not expected secret Component for %s", expectedSecret.String())
		s.Equal(expectedSecret.Status, secret.Status, "Not expected secret Status for %s", expectedSecret.String())
		s.Equal(expectedSecret.Resource, secret.Resource, "Not expected secret Resource for %s", expectedSecret.String())

		if expectedSecret.Updated == nil {
			s.Nil(secret.Updated)
		} else {
			s.WithinDuration(time.Now(), *expectedSecret.Updated, 1*time.Second, "Updated timestamp for %s", expectedSecret.Name)
		}
	}
}

func (s *secretHandlerTestSuite) prepareTestRun(scenario *getSecretScenario, appName, envName, deploymentName string) *controllertest.Utils {
	_, environmentControllerTestUtils, _, kubeClient, radixClient, _, _, secretClient, _ := setupTest(s.T(), nil)
	_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}, metav1.CreateOptions{})
	require.NoError(s.T(), err)
	appAppNamespace := operatorutils.GetAppNamespace(appName)
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: appAppNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{{Name: envName}},
			Components:   testGetRadixComponents(scenario.components, envName),
			Jobs:         testGetRadixJobComponents(scenario.jobs, envName),
		},
	}
	_, _ = radixClient.RadixV1().RadixApplications(appAppNamespace).Create(context.Background(), ra, metav1.CreateOptions{})
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	re := &v1.RadixEnvironment{
		ObjectMeta: metav1.ObjectMeta{Name: envNamespace},
		Spec:       v1.RadixEnvironmentSpec{AppName: appName, EnvName: envName},
	}
	_, err = radixClient.RadixV1().RadixEnvironments().Create(context.Background(), re, metav1.CreateOptions{})
	require.NoError(s.T(), err)

	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: v1.RadixDeploymentSpec{
			Environment: envName,
			Components:  scenario.components,
			Jobs:        scenario.jobs,
		},
		Status: v1.RadixDeployStatus{
			Condition: v1.DeploymentActive,
		},
	}
	rd, _ := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
	if scenario.init != nil {
		scenario.init(kubeClient, radixClient, secretClient) // scenario optional custom init function
	}
	for _, secret := range scenario.existingSecrets {
		_, _ = kubeClient.CoreV1().Secrets(envNamespace).Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.secretName,
				Namespace: envNamespace,
				Labels:    secret.labels,
			},
			Data: secret.secretData,
		}, metav1.CreateOptions{})
	}
	for _, component := range rd.Spec.Components {
		s.createExpectedReplicas(scenario, &component, kubeClient, appName, envNamespace, false)
	}
	for _, jobComponent := range rd.Spec.Jobs {
		s.createExpectedReplicas(scenario, &jobComponent, kubeClient, appName, envNamespace, true)
	}
	return environmentControllerTestUtils
}

func (s *secretHandlerTestSuite) createExpectedReplicas(scenario *getSecretScenario, component v1.RadixCommonDeployComponent, kubeClient kubernetes.Interface, appName, envNamespace string, isJobComponent bool) {
	// map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	if azureKeyVaultNameMap, ok := scenario.expectedSecretVersions[component.GetName()]; ok {
		replicaNameMap := make(map[string]bool)
		for _, secretIdMap := range azureKeyVaultNameMap {
			for _, versionMap := range secretIdMap {
				for _, replicaMap := range versionMap {
					for replicaName := range replicaMap {
						replicaNameMap[replicaName] = true
					}
				}
			}
		}
		for replicaName := range replicaNameMap {
			s.createPodForRadixComponent(kubeClient, appName, envNamespace, component.GetName(), replicaName, isJobComponent)
			if isJobComponent {
				s.createJobForRadixJobComponent(kubeClient, appName, envNamespace, component.GetName())
			}
		}
	}
}

func (s *secretHandlerTestSuite) createPodForRadixComponent(kubeClient kubernetes.Interface, appName, envNamespace, componentName, replicaName string, isJobComponent bool) {
	labels := map[string]string{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
	}
	if isJobComponent {
		labels[kube.RadixJobTypeLabel] = kube.RadixJobTypeJobSchedule
	}
	_, _ = kubeClient.CoreV1().Pods(envNamespace).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              replicaName,
			Namespace:         envNamespace,
			Labels:            labels,
			CreationTimestamp: metav1.Now(),
		},
	}, metav1.CreateOptions{})
}

func (s *secretHandlerTestSuite) createJobForRadixJobComponent(kubeClient kubernetes.Interface, appName, envNamespace, componentName string) {
	labels := map[string]string{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}
	_, _ = kubeClient.BatchV1().Jobs(envNamespace).Create(context.Background(), &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              componentName,
			Namespace:         envNamespace,
			Labels:            labels,
			CreationTimestamp: metav1.Now(),
		},
	}, metav1.CreateOptions{})
}

func testGetRadixComponents(components []v1.RadixDeployComponent, envName string) []v1.RadixComponent {
	var radixComponents []v1.RadixComponent
	for _, radixDeployComponent := range components {
		radixComponents = append(radixComponents, v1.RadixComponent{
			Name:       radixDeployComponent.Name,
			Identity:   &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "some-client-id"}},
			Variables:  radixDeployComponent.GetEnvironmentVariables(),
			Secrets:    radixDeployComponent.Secrets,
			SecretRefs: radixDeployComponent.SecretRefs,
			EnvironmentConfig: []v1.RadixEnvironmentConfig{{
				Environment:  envName,
				VolumeMounts: radixDeployComponent.VolumeMounts,
			}},
		})
	}
	return radixComponents
}

func testGetRadixJobComponents(jobComponents []v1.RadixDeployJobComponent, envName string) []v1.RadixJobComponent {
	var radixComponents []v1.RadixJobComponent
	for _, radixDeployJobComponent := range jobComponents {
		radixComponents = append(radixComponents, v1.RadixJobComponent{
			Name:       radixDeployJobComponent.Name,
			Identity:   &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "some-client-id"}},
			Variables:  radixDeployJobComponent.GetEnvironmentVariables(),
			Secrets:    radixDeployJobComponent.Secrets,
			SecretRefs: radixDeployJobComponent.SecretRefs,
			EnvironmentConfig: []v1.RadixJobComponentEnvironmentConfig{{
				Environment:  envName,
				VolumeMounts: radixDeployJobComponent.VolumeMounts,
			}},
		})
	}
	return radixComponents
}

func testGetSecretMap(secrets []secretModels.Secret) map[string]secretModels.Secret {
	secretMap := make(map[string]secretModels.Secret, len(secrets))
	for _, secret := range secrets {
		secret := secret
		secretMap[secret.Name] = secret
	}
	return secretMap
}

func (scenario *getSecretScenario) testSetExpectedSecretStatus(componentName, secretId string, status secretModels.SecretStatus) {
	for i, expectedSecret := range scenario.expectedSecrets {
		if strings.EqualFold(expectedSecret.Component, componentName) && strings.EqualFold(expectedSecret.ID, secretId) {
			scenario.expectedSecrets[i].Status = status.String()
			return
		}
	}
}

func testCreateAzureKeyVaultCsiDriverSecret(kubeClient kubernetes.Interface, secretName string, data map[string]string) {
	secretData := make(map[string][]byte)
	for key, value := range data {
		secretData[key] = []byte(value)
	}
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: map[string]string{"secrets-store.csi.k8s.io/managed": "true"},
		},
		Data: secretData,
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func testCreateScenario(name string, modify func(*getSecretScenario)) getSecretScenario {
	scenario := getSecretScenario{
		name: name,
	}
	modify(&scenario)
	return scenario
}

func testCreateScenarioWithComponentAndJobWithCredSecretsAndOneSecretPerComponent(name string, modify func(*getSecretScenario)) getSecretScenario {
	scenario := testCreateScenario(name, func(scenario *getSecretScenario) {
		const (
			componentName1 = "component1"
			jobName1       = "job1"
			keyVaultName1  = "keyVault1"
			keyVaultName2  = "keyVault2"
		)
		scenario.components = []v1.RadixDeployComponent{
			testCreateRadixDeployComponent(componentName1, keyVaultName1, "", "secret1", "SECRET_REF1"),
		}
		scenario.jobs = []v1.RadixDeployJobComponent{
			testCreateRadixDeployJobComponent(jobName1, keyVaultName2, "secret2", "SECRET_REF2"),
		}
		scenario.existingSecrets = []testSecretDescription{
			{
				secretName: "component1-keyvault1-csiazkvcreds",
				secretData: map[string][]byte{
					"clientid":     []byte("current client id1"),
					"clientsecret": []byte("current client secret1"),
				},
			},
			{
				secretName: "job1-keyvault2-csiazkvcreds",
				secretData: map[string][]byte{
					"clientid":     []byte("current client id2"),
					"clientsecret": []byte("current client secret2"),
				},
			},
		}
		scenario.expectedSecrets = []secretModels.Secret{
			{
				Name:        "component1-keyvault1-csiazkvcreds-azkv-clientid",
				DisplayName: "Client ID",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientId,
			},
			{
				Name:        "component1-keyvault1-csiazkvcreds-azkv-clientsecret",
				DisplayName: "Client Secret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientSecret,
			},
			{
				Name:        "AzureKeyVaultItem-component1--keyVault1--secret--secret1",
				DisplayName: "secret secret1",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
				Resource:    keyVaultName1,
				Component:   componentName1,
				Status:      secretModels.Pending.String(),
				ID:          "secret/secret1",
			},
			{
				Name:        "job1-keyvault2-csiazkvcreds-azkv-clientid",
				DisplayName: "Client ID",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientId,
			},
			{
				Name:        "job1-keyvault2-csiazkvcreds-azkv-clientsecret",
				DisplayName: "Client Secret",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultCreds,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.Pending.String(),
				ID:          secretModels.SecretIdClientSecret,
			},
			{
				Name:        "AzureKeyVaultItem-job1--keyVault2--secret--secret2",
				DisplayName: "secret secret2",
				Type:        secretModels.SecretTypeCsiAzureKeyVaultItem,
				Resource:    keyVaultName2,
				Component:   jobName1,
				Status:      secretModels.NotAvailable.String(),
				ID:          "secret/secret2",
			},
		}

	})
	modify(&scenario)
	return scenario
}

func testCreateRadixDeployComponent(componentName, keyVaultName string, secretObjectType v1.RadixAzureKeyVaultObjectType, secretName, envVarName string) v1.RadixDeployComponent {
	return v1.RadixDeployComponent{
		Name: componentName,
		SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: keyVaultName,
				Items: []v1.RadixAzureKeyVaultItem{
					{
						Name:   secretName,
						EnvVar: envVarName,
						Type:   &secretObjectType,
					},
				}},
		}},
	}
}

func testCreateRadixDeployJobComponent(componentName, keyVaultName, secretName, envVarName string) v1.RadixDeployJobComponent {
	return v1.RadixDeployJobComponent{
		Name: componentName,
		SecretRefs: v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{
			{
				Name: keyVaultName,
				Items: []v1.RadixAzureKeyVaultItem{
					{
						Name:   secretName,
						EnvVar: envVarName,
					},
				}},
		}},
	}
}

func testCreateSecretProviderClass(secretProviderClient secretsstoreclient.Interface, radixDeploymentName string, component v1.RadixCommonDeployComponent) map[string]testSecretProviderClassAndSecret {
	azureKeyVaultSecretProviderClassNameMap := make(map[string]testSecretProviderClassAndSecret)
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		secretProviderClass, err := kube.BuildAzureKeyVaultSecretProviderClass("123456789", anyAppName, radixDeploymentName, component.GetName(), azureKeyVault, component.GetIdentity())
		if err != nil {
			panic(err)
		}
		namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
		_, err = secretProviderClient.SecretsstoreV1().SecretProviderClasses(namespace).Create(context.Background(),
			secretProviderClass, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		azureKeyVaultSecretProviderClassNameMap[azureKeyVault.Name] = testSecretProviderClassAndSecret{
			secretName: secretProviderClass.Spec.SecretObjects[0].SecretName,
			className:  secretProviderClass.GetName(),
		}
	}
	return azureKeyVaultSecretProviderClassNameMap // map[componentName]map[azureKeyVaultName]createSecretProviderClassName
}
