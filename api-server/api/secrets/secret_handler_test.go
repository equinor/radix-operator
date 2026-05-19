package secrets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils"
	deployMock "github.com/equinor/radix-operator/api-server/api/deployments/mock"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/api/secrets/suffix"
	"github.com/equinor/radix-operator/api-server/api/utils/secret"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
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

type testGetSecretScenario struct {
	name                   string
	components             []v1.RadixDeployComponent
	jobs                   []v1.RadixDeployJobComponent
	init                   func(*SecretHandler) // scenario optional custom init function
	existingSecrets        []testSecretDescription
	expectedSecretVersions map[string]map[string]map[string]map[string]map[string]bool // map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
}

type testChangeSecretScenario struct {
	name                        string
	components                  []v1.RadixDeployComponent
	jobs                        []v1.RadixDeployJobComponent
	secretName                  string
	secretDataKey               string
	secretValue                 string
	secretExists                bool
	changingSecretComponentName string
	changingSecretName          string
	expectedError               bool
	changingSecretParams        secretModels.SecretParameters
	credentialsType             v1.CredentialsType
}

type testSecretProviderClassAndSecret struct {
	secretName string
	className  string
}

func (s *secretHandlerTestSuite) TestSecretHandler_GetAzureKeyVaultSecretRefStatuses() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	const (
		deployment1    = "deployment1"
		componentName1 = "component1"
		componentName2 = "component2"
		componentName3 = "component3"
		jobName1       = "job1"
		keyVaultName1  = "keyVault1"
		keyVaultName2  = "keyVault2"
		keyVaultName3  = "keyVault3"
		secret1        = "secret1"
		secret2        = "secret2"
		secret3        = "secret3"
		secretEnvVar1  = "SECRET_REF1"
		secretEnvVar2  = "SECRET_REF2"
		secretEnvVar3  = "SECRET_REF3"
	)

	scenarios := []testGetSecretScenario{
		testCreateScenario("All secret version statuses exist, but for component3",
			func(scenario *testGetSecretScenario) {
				scenario.components = []v1.RadixDeployComponent{
					testCreateRadixDeployComponent(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
					testCreateRadixDeployComponent(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, secretEnvVar3),
					testCreateRadixDeployComponent(componentName3, keyVaultName3, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
				}
				scenario.jobs = []v1.RadixDeployJobComponent{testCreateRadixDeployJobComponent(jobName1, keyVaultName2, secret2, secretEnvVar2)}
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c12", "replica-name-c11")
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c11", "replica-name-c12")
				scenario.setExpectedSecretVersion(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, "version-c3", "replica-name-c13")
				scenario.setExpectedSecretVersion(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, "version-c21", "replica-name-c21")
				scenario.setExpectedSecretVersion(jobName1, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret2, "version-j1", "replica-name-j1")
				initFunc := func(secretHandler *SecretHandler) {
					// map[componentName]map[azureKeyVaultName]secretProviderClassAndSecret
					componentAzKeyVaultSecretProviderClassNameMap := map[string]map[string]testSecretProviderClassAndSecret{
						componentName1: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0]),
						componentName2: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[1]),
						componentName3: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[2]),
						jobName1:       testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0]),
					}
					testCreateSecretProviderClassPodStatuses(secretHandler.serviceAccount.SecretProviderClient, scenario, componentAzKeyVaultSecretProviderClassNameMap)
				}
				scenario.init = initFunc
			}),
		testCreateScenario("No secret version statuses exist",
			func(scenario *testGetSecretScenario) {
				scenario.components = []v1.RadixDeployComponent{
					testCreateRadixDeployComponent(componentName1, keyVaultName1, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
					testCreateRadixDeployComponent(componentName2, keyVaultName2, v1.RadixAzureKeyVaultObjectTypeSecret, secret3, secretEnvVar3),
					testCreateRadixDeployComponent(componentName3, keyVaultName3, v1.RadixAzureKeyVaultObjectTypeSecret, secret1, secretEnvVar1),
				}
				scenario.jobs = []v1.RadixDeployJobComponent{testCreateRadixDeployJobComponent(jobName1, keyVaultName2, secret2, secretEnvVar2)}
				initFunc := func(secretHandler *SecretHandler) {
					// map[componentName]map[azureKeyVaultName]createSecretProviderClassName
					componentAzKeyVaultSecretProviderClassNameMap := map[string]map[string]testSecretProviderClassAndSecret{
						componentName1: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[0]),
						componentName2: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[1]),
						componentName3: testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.components[2]),
						jobName1:       testCreateSecretProviderClass(secretHandler.serviceAccount.SecretProviderClient, deployment1, &scenario.jobs[0]),
					}
					testCreateSecretProviderClassPodStatuses(secretHandler.serviceAccount.SecretProviderClient, scenario, componentAzKeyVaultSecretProviderClassNameMap)
				}
				scenario.init = initFunc
			}),
	}

	for _, scenario := range scenarios {
		appName := anyAppName
		environment := anyEnvironment
		deploymentName := deployment1

		s.Run(fmt.Sprintf("test GetSecretsStatus: %s", scenario.name), func() {
			secretHandler, _ := s.prepareTestRun(ctrl, &scenario, appName, environment, deploymentName)

			actualSecretVersions := make(map[string]map[string]map[string]map[string]map[string]bool) // map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
			for _, component := range scenario.components {
				s.appendActualSecretVersions(&component, secretHandler, appName, environment, actualSecretVersions)
			}
			for _, jobComponent := range scenario.jobs {
				s.appendActualSecretVersions(&jobComponent, secretHandler, appName, environment, actualSecretVersions)
			}
			s.assertSecretVersionStatuses(scenario.expectedSecretVersions, actualSecretVersions)
		})
	}
}

func (s *secretHandlerTestSuite) appendActualSecretVersions(component v1.RadixCommonDeployComponent, secretHandler SecretHandler, appName string, environment string, actualSecretVersions map[string]map[string]map[string]map[string]map[string]bool) {
	azureKeyVaultMap := make(map[string]map[string]map[string]map[string]bool) // map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		itemSecretMap := make(map[string]map[string]map[string]bool) // map[secretId]map[version]map[replicaName]bool
		for _, item := range azureKeyVault.Items {
			secretId := secret.GetSecretIdForAzureKeyVaultItem(&item)

			secretVersions, err := secretHandler.GetAzureKeyVaultSecretVersions(appName, environment, component.GetName(), azureKeyVault.Name, secretId)
			s.Nil(err)

			versionReplicaNameMap := make(map[string]map[string]bool) // map[version]map[replicaName]bool
			for _, secretVersion := range secretVersions {
				if _, ok := versionReplicaNameMap[secretVersion.Version]; !ok {
					versionReplicaNameMap[secretVersion.Version] = make(map[string]bool)
				}
				versionReplicaNameMap[secretVersion.Version][secretVersion.ReplicaName] = true
			}
			if len(versionReplicaNameMap) > 0 {
				itemSecretMap[secretId] = versionReplicaNameMap
			}
		}
		if len(itemSecretMap) > 0 {
			azureKeyVaultMap[azureKeyVault.Name] = itemSecretMap
		}
	}
	if len(azureKeyVaultMap) > 0 {
		actualSecretVersions[component.GetName()] = azureKeyVaultMap
	}
}

func (s *secretHandlerTestSuite) TestSecretHandler_ChangeSecrets() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	componentName1 := "component1"
	jobName1 := "job1"
	volumeName1 := "volume1"
	azureKeyVaultName1 := "azureKeyVault1"
	//goland:noinspection ALL
	scenarios := []testChangeSecretScenario{
		{
			name: "Change regular secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretName:                  "component1-sdiatyab",
			secretDataKey:               "SECRET_C1",
			secretValue:                 "current-value",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: false,
		},
		{
			name: "Change regular secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretName:                  "job1-jvqbisnq",
			secretDataKey:               "SECRET_C1",
			secretValue:                 "current-value",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing regular secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing regular secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
				Secrets: []string{
					"SECRET_C1",
				},
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          "SECRET_C1",
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new-value",
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Blob volume account name in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Blob volume account name in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountNamePart,
			secretValue:                 "currentAccountName",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account name in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account name in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountNamePartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountName",
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Blob volume account key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Blob volume account key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1),
			secretDataKey:               defaults.CsiAzureCredsAccountKeyPart,
			secretValue:                 "currentAccountKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(componentName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Blob volume account key in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureVolumeMountCredsSecretName(jobName1, volumeName1) + defaults.CsiAzureCredsAccountKeyPartSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newAccountKey",
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Key vault client ID in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientIdPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Key vault client ID in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientIdPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client ID in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client ID in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientIdSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: true,
		},
		{
			name: "Change CSI Azure Key vault client secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientSecretPart,
			secretValue:                 "currentClientId",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: false,
		},
		{
			name: "Change CSI Azure Key vault client secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretName:                  defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1),
			secretDataKey:               defaults.CsiAzureKeyVaultCredsClientSecretPart,
			secretValue:                 "currentClientSecret",
			secretExists:                true,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecret",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(componentName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientId",
			},
			expectedError: true,
		},
		{
			name: "Failed change of not existing CSI Azure Key vault client secret in the job",
			jobs: []v1.RadixDeployJobComponent{{
				Name: jobName1,
			}},
			secretExists:                false,
			changingSecretComponentName: jobName1,
			changingSecretName:          defaults.GetCsiAzureKeyVaultCredsSecretName(jobName1, azureKeyVaultName1) + defaults.CsiAzureKeyVaultCredsClientSecretSuffix,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecret",
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 client secret key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthClientSecretKeyName,
			secretValue:                 "currentClientSecretKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
			},
			expectedError: false,
		},
		{
			name: "Failed to change OAuth2 client secret key in the component with OAuth2 using identity",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthClientSecretKeyName,
			secretValue:                 "currentClientSecretKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
			},
			expectedError:   true,
			credentialsType: v1.AzureWorkloadIdentity,
		},
		{
			name: "Failed change of not existing OAuth2 client secret key in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2ClientSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newClientSecretKey",
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 cookie secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthCookieSecretKeyName,
			secretValue:                 "currentCookieSecretKey",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing OAuth2 cookie secret in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2CookieSecret,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newCookieSecretKey",
			},
			expectedError: true,
		},
		{
			name: "Change OAuth2 Redis password in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix),
			secretDataKey:               defaults.OAuthRedisPasswordKeyName,
			secretValue:                 "currentRedisPassword",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing OAuth2 Redis password in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          operatorUtils.GetAuxiliaryComponentSecretName(componentName1, v1.OAuthProxyAuxiliaryComponentSuffix) + suffix.OAuth2RedisPassword,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "newRedisPassword",
			},
			expectedError: true,
		},
		{
			name: "Change client certificate in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretName:                  "client-certificate1-clientcertca",
			secretDataKey:               "ca.crt",
			secretValue:                 "current client certificate\nline2\nline3",
			secretExists:                true,
			changingSecretComponentName: componentName1,
			changingSecretName:          "client-certificate1" + suffix.ClientCertificate,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new client certificate\nline2\nline3",
			},
			expectedError: false,
		},
		{
			name: "Failed change of not existing client certificate in the component",
			components: []v1.RadixDeployComponent{{
				Name: componentName1,
			}},
			secretExists:                false,
			changingSecretComponentName: componentName1,
			changingSecretName:          "client-certificate1" + suffix.ClientCertificate,
			changingSecretParams: secretModels.SecretParameters{
				SecretValue: "new client certificate\nline2\nline3",
			},
			expectedError: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(fmt.Sprintf("test GetSecrets: %s", scenario.name), func() {
			appName := anyAppName
			envName := anyEnvironment
			userAccount, serviceAccount, kubeClient, radixClient := s.getUtils()
			secretHandler := SecretHandler{
				userAccount:    *userAccount,
				serviceAccount: *serviceAccount,
			}
			appEnvNamespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
			if scenario.secretExists {
				_, _ = kubeClient.CoreV1().Secrets(appEnvNamespace).Create(context.Background(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: scenario.secretName, Namespace: appEnvNamespace},
					Data:       map[string][]byte{scenario.secretDataKey: []byte(scenario.secretValue)},
				}, metav1.CreateOptions{})
			}
			_, err := radixClient.RadixV1().RadixDeployments(appEnvNamespace).Create(context.Background(), createActiveRadixDeployment(envName, componentName1, scenario.credentialsType), metav1.CreateOptions{})
			s.NoError(err, "Failed to create RadixDeployment")

			err = secretHandler.ChangeComponentSecret(context.Background(), appName, envName, scenario.changingSecretComponentName, scenario.changingSecretName, scenario.changingSecretParams)

			s.Equal(scenario.expectedError, err != nil, testGetErrorMessage(err))
			if scenario.secretExists && err == nil {
				changedSecret, _ := kubeClient.CoreV1().Secrets(appEnvNamespace).Get(context.Background(), scenario.secretName, metav1.GetOptions{})
				s.NotNil(changedSecret)
				s.Equal(scenario.changingSecretParams.SecretValue, string(changedSecret.Data[scenario.secretDataKey]))
				secretUpdatedAt := kubequery.GetSecretMetadata(context.Background(), changedSecret).GetUpdated(scenario.secretDataKey)
				require.NotNil(s.T(), secretUpdatedAt)
				s.WithinDuration(time.Now(), *secretUpdatedAt, 1*time.Second)
			}
		})
	}
}

func createActiveRadixDeployment(envName, componentName string, credentialsType v1.CredentialsType) *v1.RadixDeployment {
	deployComponent := v1.RadixDeployComponent{Name: componentName, Image: "comp_image1"}
	if credentialsType != "" {
		deployComponent.Authentication = &v1.Authentication{
			OAuth2: &v1.OAuth2{Credentials: credentialsType},
		}
		deployComponent.Identity = &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "some-client-id"}}
	}
	return &v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "deployment-name"},
		Spec: v1.RadixDeploymentSpec{
			Environment: envName,
			Components:  []v1.RadixDeployComponent{deployComponent},
		},
		Status: v1.RadixDeployStatus{
			ActiveFrom: metav1.NewTime(time.Now().Truncate(24 * time.Hour)),
			Condition:  v1.DeploymentActive,
		},
	}
}

func testGetErrorMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func (s *secretHandlerTestSuite) assertSecretVersionStatuses(expectedVersionsMap map[string]map[string]map[string]map[string]map[string]bool, actualVersionsMap map[string]map[string]map[string]map[string]map[string]bool) {
	// maps: map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	s.Equal(len(expectedVersionsMap), len(actualVersionsMap), "Not equal component count")
	for componentName, actualAzureKeyVaultMap := range actualVersionsMap {
		expectedAzureKeyVaultMap, ok := expectedVersionsMap[componentName]
		s.True(ok, "Missing AzureKeyVaults for the component %s", componentName)
		s.Equal(len(expectedAzureKeyVaultMap), len(actualAzureKeyVaultMap), "Not equal AzureKeyVaults count for the component %s", componentName)
		for azKeyVaultName, actualItemsMap := range actualAzureKeyVaultMap {
			expectedItemsMap, ok := expectedAzureKeyVaultMap[azKeyVaultName]
			s.True(ok, "Missing AzureKeyVault items for the component %s, Azure Key vault %s", componentName, azKeyVaultName)
			s.Equal(len(expectedItemsMap), len(actualItemsMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s", componentName, azKeyVaultName)
			for secretId, actualVersionsMap := range actualItemsMap {
				expectedVersionsMap, ok := expectedItemsMap[secretId]
				s.True(ok, "Missing AzureKeyVault item secretId for the component %s, Azure Key vault %s secretId %s", componentName, azKeyVaultName, secretId)
				s.Equal(len(expectedVersionsMap), len(actualVersionsMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s secretId %s", componentName, azKeyVaultName, secretId)
				for version, actualReplicaNamesMap := range actualVersionsMap {
					expectedReplicaNamesMap, ok := expectedVersionsMap[version]
					s.True(ok, "Missing AzureKeyVault item secretId version for the component %s, Azure Key vault %s secretId %s version %s", componentName, azKeyVaultName, secretId, version)
					s.Equal(len(expectedReplicaNamesMap), len(actualReplicaNamesMap), "Not equal AzureKeyVault items count for the component %s, Azure Key vault %s secretId %s version %s", componentName, azKeyVaultName, secretId, version)
					for replicaName := range actualReplicaNamesMap {
						_, ok := expectedReplicaNamesMap[replicaName]
						s.True(ok, "Missing AzureKeyVault item secretId version replica for the component %s, Azure Key vault %s secretId %s version %s replicaName %s", componentName, azKeyVaultName, secretId, version, replicaName)
					}
				}
			}
		}
	}
}

func (s *secretHandlerTestSuite) prepareTestRun(ctrl *gomock.Controller, scenario *testGetSecretScenario, appName, envName, deploymentName string) (SecretHandler, *deployMock.MockDeployHandler) {
	userAccount, serviceAccount, kubeClient, radixClient := s.getUtils()
	deployHandler := deployMock.NewMockDeployHandler(ctrl)
	secretHandler := SecretHandler{
		userAccount:    *userAccount,
		serviceAccount: *serviceAccount,
		deployHandler:  deployHandler,
	}
	appAppNamespace := operatorUtils.GetAppNamespace(appName)
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: appAppNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{{Name: envName}},
			Components:   testGetRadixComponents(scenario.components, envName),
			Jobs:         testGetRadixJobComponents(scenario.jobs, envName),
		},
	}
	_, _ = radixClient.RadixV1().RadixApplications(appAppNamespace).Create(context.Background(), ra, metav1.CreateOptions{})
	radixDeployment := v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: v1.RadixDeploymentSpec{
			Environment: envName,
			Components:  scenario.components,
			Jobs:        scenario.jobs,
		},
	}
	envNamespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	rd, _ := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.Background(), &radixDeployment, metav1.CreateOptions{})
	if scenario.init != nil {
		scenario.init(&secretHandler) // scenario optional custom init function
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
		s.createExpectedReplicas(scenario, &component, userAccount, appName, envNamespace, false)
	}
	for _, jobComponent := range rd.Spec.Jobs {
		s.createExpectedReplicas(scenario, &jobComponent, userAccount, appName, envNamespace, true)
	}
	return secretHandler, deployHandler
}

func (s *secretHandlerTestSuite) createExpectedReplicas(scenario *testGetSecretScenario, component v1.RadixCommonDeployComponent, userAccount *models.Account, appName, envNamespace string, isJobComponent bool) {
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
			s.createPodForRadixComponent(userAccount, appName, envNamespace, component.GetName(), replicaName, isJobComponent)
			if isJobComponent {
				s.createJobForRadixJobComponent(userAccount, appName, envNamespace, component.GetName())
			}
		}
	}
}

func (s *secretHandlerTestSuite) createPodForRadixComponent(userAccount *models.Account, appName, envNamespace, componentName, replicaName string, isJobComponent bool) {
	labels := map[string]string{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
	}
	if isJobComponent {
		labels[k8sJobNameLabel] = componentName
		labels[kube.RadixJobTypeLabel] = kube.RadixJobTypeJobSchedule
	}
	_, _ = userAccount.Client.CoreV1().Pods(envNamespace).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              replicaName,
			Namespace:         envNamespace,
			Labels:            labels,
			CreationTimestamp: metav1.Now(),
		},
	}, metav1.CreateOptions{})
}

func (s *secretHandlerTestSuite) createJobForRadixJobComponent(userAccount *models.Account, appName, envNamespace, componentName string) {
	labels := map[string]string{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}
	_, _ = userAccount.Client.BatchV1().Jobs(envNamespace).Create(context.Background(), &batchv1.Job{
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

func (s *secretHandlerTestSuite) getUtils() (*models.Account, *models.Account, *kubefake.Clientset, *radixfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	userAccount := models.Account{
		Client:               kubeClient,
		RadixClient:          radixClient,
		SecretProviderClient: secretProviderClient,
	}
	serviceAccount := models.Account{
		Client:               kubeClient,
		RadixClient:          radixClient,
		SecretProviderClient: secretProviderClient,
	}
	return &userAccount, &serviceAccount, kubeClient, radixClient
}

func (scenario *testGetSecretScenario) setExpectedSecretVersion(componentName, azureKeyVaultName string, secretType v1.RadixAzureKeyVaultObjectType, secretName, version, replicaName string) {
	secretId := fmt.Sprintf("%s/%s", string(secretType), secretName)
	if scenario.expectedSecretVersions == nil {
		scenario.expectedSecretVersions = make(map[string]map[string]map[string]map[string]map[string]bool) // map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	}
	if _, ok := scenario.expectedSecretVersions[componentName]; !ok {
		scenario.expectedSecretVersions[componentName] = make(map[string]map[string]map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName] = make(map[string]map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId] = make(map[string]map[string]bool)
	}
	if _, ok := scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version]; !ok {
		scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version] = make(map[string]bool)
	}
	scenario.expectedSecretVersions[componentName][azureKeyVaultName][secretId][version][replicaName] = true
}

func testCreateScenario(name string, modify func(*testGetSecretScenario)) testGetSecretScenario {
	scenario := testGetSecretScenario{
		name: name,
	}
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

func testCreateSecretProviderClass(secretProviderClient secretProviderClient.Interface, radixDeploymentName string, component v1.RadixCommonDeployComponent) map[string]testSecretProviderClassAndSecret {
	azureKeyVaultSecretProviderClassNameMap := make(map[string]testSecretProviderClassAndSecret)
	for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
		secretProviderClass, err := kube.BuildAzureKeyVaultSecretProviderClass("123456789", anyAppName, radixDeploymentName, component.GetName(), azureKeyVault, component.GetIdentity())
		if err != nil {
			panic(err)
		}
		namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
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

func testCreateSecretProviderClassPodStatuses(secretProviderClient secretProviderClient.Interface, scenario *testGetSecretScenario, componentAzKeyVaultSecretProviderClassNameMap map[string]map[string]testSecretProviderClassAndSecret) {
	namespace := operatorUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	// scenario.expectedSecretVersions: map[componentName]map[azureKeyVaultName]map[secretId]map[version]map[replicaName]bool
	for componentName, azKeyVaultMap := range scenario.expectedSecretVersions {
		// componentAzKeyVaultSecretProviderClassNameMap map[componentName]map[AzureKeyVault]SecretName
		secretProviderClassNameMap, ok := componentAzKeyVaultSecretProviderClassNameMap[componentName]
		if !ok {
			continue
		}
		for azKeyVaultName, secretIdMap := range azKeyVaultMap {
			secretProviderClassAndSecret, ok := secretProviderClassNameMap[azKeyVaultName]
			if !ok {
				continue
			}
			classObjectsMap := testGetReplicaNameToSecretProviderClassObjectsMap(secretIdMap)
			testCreateSecretProviderClassPodStatusesForAzureKeyVault(secretProviderClient, namespace, classObjectsMap, secretProviderClassAndSecret.className)
		}
	}
}

func testCreateSecretProviderClassPodStatusesForAzureKeyVault(secretProviderClient secretProviderClient.Interface, namespace string, secretProviderClassObjectsMap map[string][]secretsstorev1.SecretProviderClassObject, secretProviderClassName string) {
	// secretProviderClassObjects map[replicaName]SecretProviderClassObject
	for replicaName, secretProviderClassObjects := range secretProviderClassObjectsMap {
		_, err := secretProviderClient.SecretsstoreV1().SecretProviderClassPodStatuses(namespace).Create(context.Background(),
			&secretsstorev1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{Name: utils.RandString(10)}, // Name is not important
				Status: secretsstorev1.SecretProviderClassPodStatusStatus{
					PodName:                 replicaName,
					SecretProviderClassName: secretProviderClassName,
					Objects:                 secretProviderClassObjects, // Secret id/version pairs
				},
			}, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}
}

func testGetReplicaNameToSecretProviderClassObjectsMap(secretIdMap map[string]map[string]map[string]bool) map[string][]secretsstorev1.SecretProviderClassObject {
	// secretIdMap: map[secretId]map[version]map[replicaName]bool
	objectsMap := make(map[string][]secretsstorev1.SecretProviderClassObject) // map[replicaName]SecretProviderClassObject
	for secretId, versionReplicaNameMap := range secretIdMap {
		for version, replicaNameMap := range versionReplicaNameMap {
			for replicaName := range replicaNameMap {
				if _, ok := objectsMap[replicaName]; !ok {
					objectsMap[replicaName] = []secretsstorev1.SecretProviderClassObject{}
				}
				objectsMap[replicaName] = append(objectsMap[replicaName], secretsstorev1.SecretProviderClassObject{
					ID:      secretId,
					Version: version,
				})
			}
		}
	}
	return objectsMap
}
