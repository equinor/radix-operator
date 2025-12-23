package deployment

import (
	"context"
	"fmt"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type OAuthProxyResourceManagerTestSuite struct {
	suite.Suite
	kubeClient                kubernetes.Interface
	radixClient               radixclient.Interface
	kedaClient                kedav2.Interface
	secretProviderClient      secretProviderClient.Interface
	kubeUtil                  *kube.Kube
	ctrl                      *gomock.Controller
	ingressAnnotationProvider *ingress.MockAnnotationProvider
	oauth2Config              *defaults.MockOAuth2Config
	clusterName               string
	dnsZone                   string
	appAliasDnsZone           string
}

func TestOAuthProxyResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthProxyResourceManagerTestSuite))
}

func (s *OAuthProxyResourceManagerTestSuite) SetupSuite() {
	s.dnsZone = "dev.radix.equinor.com"
	s.appAliasDnsZone = "app.dev.radix.equinor.com"
	s.T().Setenv(defaults.OperatorDNSZoneEnvironmentVariable, s.dnsZone)
	s.T().Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, s.appAliasDnsZone)
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	s.T().Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	s.T().Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	s.T().Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "docker.io/radix-job-scheduler:main-latest")
	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
	s.T().Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, "oidc_issuer_url")
}

func (s *OAuthProxyResourceManagerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *OAuthProxyResourceManagerTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *OAuthProxyResourceManagerTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	s.ctrl = gomock.NewController(s.T())
	s.ingressAnnotationProvider = ingress.NewMockAnnotationProvider(s.ctrl)
	s.oauth2Config = defaults.NewMockOAuth2Config(s.ctrl)
	s.clusterName = "any-cluster"
	handlerTestUtils := test.NewTestUtils(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	if err := handlerTestUtils.CreateClusterPrerequisites(s.clusterName, ""); err != nil {
		panic(fmt.Errorf("failed to setup test: %w", err))
	}
}

func (s *OAuthProxyResourceManagerTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *OAuthProxyResourceManagerTestSuite) TestNewOAuthProxyResourceManager() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	oauthConfig := defaults.NewMockOAuth2Config(ctrl)
	rd := utils.NewDeploymentBuilder().BuildRD()
	rr := utils.NewRegistrationBuilder().BuildRR()
	ingressAnnotations := []ingress.AnnotationProvider{&ingress.MockAnnotationProvider{}}
	ingressAnnotationsProxyMode := []ingress.AnnotationProvider{&ingress.MockAnnotationProvider{}, &ingress.MockAnnotationProvider{}}

	oauthManager := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, oauthConfig, ingressAnnotations, ingressAnnotationsProxyMode, "proxy:123", "somesecret")
	sut, ok := oauthManager.(*oauthProxyResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
	s.Equal(oauthConfig, sut.oauth2DefaultConfig)
	s.Equal(ingressAnnotations, sut.ingressAnnotations)
	s.Equal(ingressAnnotationsProxyMode, sut.ingressAnnotationsProxyMode)
	s.Equal("proxy:123", sut.oauth2ProxyDockerImage)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_ComponentRestartEnvVar() {
	const (
		appName = "anyapp"
		envName = "qa"
	)
	auth := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "1234"}}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}

	tests := map[string]struct {
		rd                  *radixv1.RadixDeployment
		expectRestartEnvVar bool
	}{
		"component default config": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithEnvironmentVariable(defaults.RadixRestartEnvironmentVariable, "anyval")).
				BuildRD(),
			expectRestartEnvVar: true,
		},
		"component replicas set to 1": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectRestartEnvVar: false,
		},
	}
	for name, test := range tests {
		s.Run(name, func() {
			s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
			sut := NewOAuthProxyResourceManager(test.rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.Sync(context.Background()))

			deploys, _ := s.kubeClient.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
			envVarExist := slice.Any(deploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool { return ev.Name == defaults.RadixRestartEnvironmentVariable })
			s.Equal(test.expectRestartEnvVar, envVarExist)
		})
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_Uninstall() {
	const (
		appName = "anyapp"
		envName = "qa"
	)

	namespace := utils.GetEnvironmentNamespace(appName, envName)
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	tests := map[string]struct {
		testComponentModifier func(component utils.DeployComponentBuilder) utils.DeployComponentBuilder
		expectUninstall       bool
	}{
		"no changes": {
			testComponentModifier: func(component utils.DeployComponentBuilder) utils.DeployComponentBuilder { return component },
			expectUninstall:       false,
		},
		"missing oauth": {
			testComponentModifier: func(component utils.DeployComponentBuilder) utils.DeployComponentBuilder {
				return component.WithAuthentication(&radixv1.Authentication{OAuth2: nil})
			},
			expectUninstall: true,
		},
		"not public": {
			testComponentModifier: func(component utils.DeployComponentBuilder) utils.DeployComponentBuilder {
				return component.WithPublicPort("")
			},
			expectUninstall: true,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			// Fixture
			s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
			component := utils.NewDeployComponentBuilder().WithName("any").WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}).
				WithDNSAppAlias(true).
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo1"}, radixv1.RadixDeployExternalDNS{FQDN: "foo2"})
			rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(component)
			sut := NewOAuthProxyResourceManager(rd.BuildRD(), rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.Sync(context.Background()))
			deploys, _ := s.kubeClient.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(deploys.Items, 1)
			services, _ := s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(services.Items, 1)
			ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(ingresses.Items, 5)
			secrets, _ := s.kubeClient.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(secrets.Items, 1)
			roles, _ := s.kubeClient.RbacV1().Roles(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(roles.Items, 2)
			rolebindings, _ := s.kubeClient.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(rolebindings.Items, 2)

			// Test
			component = test.testComponentModifier(component)
			rd = rd.WithComponents(component)
			sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.Sync(context.Background()))
			deploys, _ = s.kubeClient.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(deploys.Items)
			} else {
				s.Len(deploys.Items, 1)
			}
			services, _ = s.kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(services.Items)
			} else {
				s.Len(services.Items, 1)
			}
			ingresses, _ = s.kubeClient.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(ingresses.Items)
			} else {
				s.Len(ingresses.Items, 5)
			}
			secrets, _ = s.kubeClient.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(secrets.Items)
			} else {
				s.Len(secrets.Items, 1)
			}
			roles, _ = s.kubeClient.RbacV1().Roles(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(roles.Items)
			} else {
				s.Len(roles.Items, 2)
			}
			rolebindings, _ = s.kubeClient.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})
			if test.expectUninstall {
				s.Empty(rolebindings.Items)
			} else {
				s.Len(rolebindings.Items, 2)
			}
		})

	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_UseClientSecretOrIdentity() {
	const (
		appName                    = "anyapp"
		componentAzureClientId     = "some-azure-client-id"
		oauth2ClientId             = "oauth2-client-id"
		componentName1             = "comp1"
		expectedServiceAccountName = "comp1-aux-oauth-sa"
		envName                    = "qa"
	)
	identity := &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: componentAzureClientId}}
	auxOAuthServiceAccount := buildServiceAccount(utils.GetEnvironmentNamespace(appName, envName), expectedServiceAccountName, radixlabels.ForOAuthProxyComponentServiceAccount(&radixv1.RadixDeployComponent{Name: componentName1}), oauth2ClientId)

	scenarios := map[string]struct {
		rd                          *radixv1.RadixDeployment
		expectedSa                  *corev1.ServiceAccount
		expectedAuxOauthDeployCount int
		existingSa                  *corev1.ServiceAccount
	}{
		"Not created service account without Authentication": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: componentAzureClientId}})).
				BuildRD(),
			expectedAuxOauthDeployCount: 0},
		"Not created service account with Authentication, no OAuth2": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: componentAzureClientId}}).WithAuthentication(&radixv1.Authentication{})).
				BuildRD(),
			expectedAuxOauthDeployCount: 0},
		"Not created service account with Authentication, OAuth2, false useAzureIdentity": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: componentAzureClientId}}).WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: oauth2ClientId}})).
				BuildRD(),
			expectedAuxOauthDeployCount: 1,
		},
		"Created the service account with Authentication, OAuth2, true useAzureIdentity": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(identity).WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: oauth2ClientId, Credentials: radixv1.AzureWorkloadIdentity}})).
				BuildRD(),
			expectedAuxOauthDeployCount: 1,
			expectedSa:                  auxOAuthServiceAccount,
		},
		"Delete existing service account, with Authentication, OAuth2, no useAzureIdentity": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: componentAzureClientId}}).WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: oauth2ClientId}})).
				BuildRD(),
			expectedAuxOauthDeployCount: 1,
			existingSa:                  auxOAuthServiceAccount,
		},
		"Not overridden the existing service account with Authentication, OAuth2, true useAzureIdentity": {
			rd: utils.NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(identity).WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: oauth2ClientId, Credentials: radixv1.AzureWorkloadIdentity}})).
				BuildRD(),
			expectedAuxOauthDeployCount: 1,
			existingSa:                  auxOAuthServiceAccount,
			expectedSa:                  auxOAuthServiceAccount,
		},
	}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for name, scenario := range scenarios {
		s.Run(name, func() {
			sut := &oauthProxyResourceManager{scenario.rd, rr, s.kubeUtil, nil, nil, defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()), "", "", zerolog.Nop()}
			if scenario.existingSa != nil {
				_, err := s.kubeClient.CoreV1().ServiceAccounts(scenario.existingSa.Namespace).Create(context.Background(), scenario.existingSa, metav1.CreateOptions{})
				s.NoError(err, "Failed to create service account")
			}
			auxOAuthSecret, err := sut.buildOAuthProxySecret(appName, &scenario.rd.Spec.Components[0])
			auxOAuthSecret.SetNamespace(scenario.rd.Namespace)
			s.NoError(err, "Failed to build secret")
			auxOAuthSecret.Data[defaults.OAuthClientSecretKeyName] = []byte("some-client-secret")
			_, err = s.kubeUtil.KubeClient().CoreV1().Secrets(scenario.rd.Namespace).Create(context.Background(), auxOAuthSecret, metav1.CreateOptions{})
			s.NoError(err, "Failed to create secret")

			s.Require().NoError(sut.Sync(context.Background()))

			deploys, err := s.kubeClient.AppsV1().Deployments(scenario.rd.Namespace).List(context.Background(), metav1.ListOptions{})
			s.NoError(err, "Failed to list deployments")
			s.Len(deploys.Items, scenario.expectedAuxOauthDeployCount, "Expected aux oauth deployment count")
			saList, err := s.kubeClient.CoreV1().ServiceAccounts(scenario.rd.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: radixlabels.ForOAuthProxyComponentServiceAccount(&radixv1.RadixDeployComponent{Name: componentName1}).String()})
			s.NoError(err, "Failed to list service accounts")
			existsClientSecretEnvVar := false
			existsOAuthProxyDeployment := len(deploys.Items) > 0
			if existsOAuthProxyDeployment {
				auxOAuthDeployment := deploys.Items[0]
				auxOAuthContainer := auxOAuthDeployment.Spec.Template.Spec.Containers[0]
				secretName := utils.GetAuxiliaryComponentSecretName(scenario.rd.Spec.Components[0].GetName(), radixv1.OAuthProxyAuxiliaryComponentSuffix)
				existsClientSecretEnvVar = slice.Any(auxOAuthContainer.Env, func(ev corev1.EnvVar) bool {
					return ev.Name == oauth2ProxyClientSecretEnvironmentVariable && ev.ValueFrom != nil && ev.ValueFrom.SecretKeyRef != nil &&
						ev.ValueFrom.SecretKeyRef.Key == defaults.OAuthClientSecretKeyName &&
						ev.ValueFrom.SecretKeyRef.LocalObjectReference.Name == secretName
				})
				actualSecret, err := s.kubeClient.CoreV1().Secrets(scenario.rd.Namespace).Get(context.Background(), auxOAuthSecret.GetName(), metav1.GetOptions{})
				s.NoError(err, "Failed to get aux OAuth secret")
				_, existsOAuthClientSecret := actualSecret.Data[defaults.OAuthClientSecretKeyName]
				azureWorkloadIdentityUse, existsAzureWorkloadIdentityUse := auxOAuthDeployment.Spec.Template.GetLabels()["azure.workload.identity/use"]
				if scenario.expectedSa != nil {
					s.False(existsOAuthClientSecret, "Unexpected client secret")
					s.Equal("true", azureWorkloadIdentityUse, "Expected azure workload identity use label")
				} else {
					s.True(existsOAuthClientSecret, "Expected client secret")
					s.False(existsAzureWorkloadIdentityUse, "Unexpected azure workload identity use label")
				}
			}
			if scenario.expectedSa != nil {
				s.Len(saList.Items, 1, fmt.Sprintf("Expected service account %s", scenario.expectedSa.GetName()))
				s.Equal(scenario.expectedSa, &saList.Items[0], fmt.Sprintf("Expected service account %s", scenario.expectedSa.GetName()))
				if existsOAuthProxyDeployment {
					s.False(existsClientSecretEnvVar, "Unexpected client secret env var")
				}
			} else {
				s.Len(saList.Items, 0, "Expected no service account")
				if existsOAuthProxyDeployment {
					s.True(existsClientSecretEnvVar, "Expected client secret env var")
				}
			}
		})
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_Oauth_DeploymentReplicas() {
	const (
		appName = "anyapp"
		envName = "qa"
	)

	auth := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "1234"}}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}

	tests := map[string]struct {
		rd               *radixv1.RadixDeployment
		expectedReplicas int32
	}{
		"component default config": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp()).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component replicas set to 1": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component replicas set to 2": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2))).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component replicas set to 0": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		"component replicas set override to 0": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		"component with hpa and default config": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component with hpa and replicas set to 1": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component with hpa and replicas set to 2": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		"component with hpa and replicas set to 0": {
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 0,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
			sut := NewOAuthProxyResourceManager(test.rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.Sync(context.Background()))

			deploys, _ := s.kubeClient.AppsV1().Deployments(test.rd.Namespace).List(context.Background(), metav1.ListOptions{})
			s.Equal(test.expectedReplicas, *deploys.Items[0].Spec.Replicas)
		})
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxy_DeploymentCreated() {
	appName, envName, componentName, oauthProxyImage := "anyapp", "qa", "server", "anyoautproxyimage"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	inputOAuth := &radixv1.OAuth2{ClientID: "1234"}
	returnOAuth := &radixv1.OAuth2{
		ClientID:               commonUtils.RandString(20),
		Scope:                  commonUtils.RandString(20),
		SetXAuthRequestHeaders: pointers.Ptr(true),
		SetAuthorizationHeader: pointers.Ptr(false),
		ProxyPrefix:            commonUtils.RandString(20),
		LoginURL:               commonUtils.RandString(20),
		RedeemURL:              commonUtils.RandString(20),
		SessionStoreType:       "redis",
		Cookie: &radixv1.OAuth2Cookie{
			Name:     commonUtils.RandString(20),
			Expire:   commonUtils.RandString(20),
			Refresh:  commonUtils.RandString(20),
			SameSite: radixv1.CookieSameSiteType(commonUtils.RandString(20)),
		},
		CookieStore: &radixv1.OAuth2CookieStore{
			Minimal: pointers.Ptr(true),
		},
		RedisStore: &radixv1.OAuth2RedisStore{
			ConnectionURL: commonUtils.RandString(20),
		},
		OIDC: &radixv1.OAuth2OIDC{
			IssuerURL:               commonUtils.RandString(20),
			JWKSURL:                 commonUtils.RandString(20),
			SkipDiscovery:           pointers.Ptr(true),
			InsecureSkipVerifyNonce: pointers.Ptr(false),
		},
		SkipAuthRoutes: []string{"POST=^/api/public-entity/?$", "GET=^/skip/auth/routes/get", "!=^/api"},
	}
	s.oauth2Config.EXPECT().MergeWith(inputOAuth).Times(1).Return(returnOAuth, nil)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: inputOAuth}).WithRuntime(&radixv1.Runtime{Architecture: "customarch"})).
		BuildRD()

	sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, oauthProxyImage, "")
	s.Require().NoError(sut.Sync(context.Background()))

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)

	actualDeploy := actualDeploys.Items[0]
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix), actualDeploy.Name)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualDeploy.OwnerReferences)

	expectedDeployLabels := map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType,
	}
	expectedPodLabels := map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAppIDLabel:                  rr.Spec.AppID.String(),
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType,
	}
	s.Equal(expectedDeployLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedPodLabels, actualDeploy.Spec.Template.Labels)

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(oauthProxyImage, defaultContainer.Image)

	s.Len(defaultContainer.Ports, 1)
	s.Equal(defaults.OAuthProxyPortNumber, defaultContainer.Ports[0].ContainerPort)
	s.Equal(defaults.OAuthProxyPortName, defaultContainer.Ports[0].Name)
	s.NotNil(defaultContainer.ReadinessProbe)
	s.Equal(defaults.OAuthProxyPortNumber, defaultContainer.ReadinessProbe.TCPSocket.Port.IntVal)

	expectedAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
			{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
			{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
		}}}}},
	}
	s.Equal(expectedAffinity, actualDeploy.Spec.Template.Spec.Affinity, "oauth2 aux deployment must not use component's runtime config")

	s.Len(defaultContainer.Env, 31)
	s.Equal("oidc", s.getEnvVarValueByName("OAUTH2_PROXY_PROVIDER", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_HTTPONLY", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_SECURE", defaultContainer.Env))
	s.Equal("false", s.getEnvVarValueByName("OAUTH2_PROXY_PASS_BASIC_AUTH", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SKIP_CLAIMS_FROM_PROFILE_URL", defaultContainer.Env))
	s.Equal("*", s.getEnvVarValueByName("OAUTH2_PROXY_EMAIL_DOMAINS", defaultContainer.Env))
	s.Equal(fmt.Sprintf("http://:%v", defaults.OAuthProxyPortNumber), s.getEnvVarValueByName("OAUTH2_PROXY_HTTP_ADDRESS", defaultContainer.Env))
	s.Equal(returnOAuth.ClientID, s.getEnvVarValueByName("OAUTH2_PROXY_CLIENT_ID", defaultContainer.Env))
	s.Equal(returnOAuth.Scope, s.getEnvVarValueByName("OAUTH2_PROXY_SCOPE", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SET_XAUTHREQUEST", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_PASS_ACCESS_TOKEN", defaultContainer.Env))
	s.Equal("false", s.getEnvVarValueByName("OAUTH2_PROXY_SET_AUTHORIZATION_HEADER", defaultContainer.Env))
	s.Equal("/"+returnOAuth.ProxyPrefix, s.getEnvVarValueByName("OAUTH2_PROXY_PROXY_PREFIX", defaultContainer.Env))
	s.Equal(returnOAuth.LoginURL, s.getEnvVarValueByName("OAUTH2_PROXY_LOGIN_URL", defaultContainer.Env))
	s.Equal(returnOAuth.RedeemURL, s.getEnvVarValueByName("OAUTH2_PROXY_REDEEM_URL", defaultContainer.Env))
	s.Equal("redis", s.getEnvVarValueByName("OAUTH2_PROXY_SESSION_STORE_TYPE", defaultContainer.Env))
	s.Equal(returnOAuth.OIDC.IssuerURL, s.getEnvVarValueByName("OAUTH2_PROXY_OIDC_ISSUER_URL", defaultContainer.Env))
	s.Equal(returnOAuth.OIDC.JWKSURL, s.getEnvVarValueByName("OAUTH2_PROXY_OIDC_JWKS_URL", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SKIP_OIDC_DISCOVERY", defaultContainer.Env))
	s.Equal("false", s.getEnvVarValueByName("OAUTH2_PROXY_INSECURE_OIDC_SKIP_NONCE", defaultContainer.Env))
	s.Equal(returnOAuth.Cookie.Name, s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_NAME", defaultContainer.Env))
	s.Equal(returnOAuth.Cookie.Expire, s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_EXPIRE", defaultContainer.Env))
	s.Equal(returnOAuth.Cookie.Refresh, s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_REFRESH", defaultContainer.Env))
	s.Equal(string(returnOAuth.Cookie.SameSite), s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_SAMESITE", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SESSION_COOKIE_MINIMAL", defaultContainer.Env))
	s.Equal(returnOAuth.RedisStore.ConnectionURL, s.getEnvVarValueByName("OAUTH2_PROXY_REDIS_CONNECTION_URL", defaultContainer.Env))
	s.Equal("POST=^/api/public-entity/?$,GET=^/skip/auth/routes/get,!=^/api", s.getEnvVarValueByName(oauth2ProxySkipAuthRoutesEnvironmentVariable, defaultContainer.Env))
	secretName := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthCookieSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyCookieSecretEnvironmentVariable, defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthClientSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyClientSecretEnvironmentVariable, defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthRedisPasswordKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyRedisPasswordEnvironmentVariable, defaultContainer.Env))

	// Env var OAUTH2_PROXY_REDIS_PASSWORD and OAUTH2_PROXY_REDIS_CONNECTION_URL should not be present when SessionStoreType is cookie
	returnOAuth.SessionStoreType = "cookie"
	s.oauth2Config.EXPECT().MergeWith(inputOAuth).Times(1).Return(returnOAuth, nil)
	s.Require().NoError(sut.Sync(context.Background()))

	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, 29, "Unexpected amount of env-vars")
	s.False(s.getEnvVarExist(oauth2ProxyRedisPasswordEnvironmentVariable, actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie")
	s.False(s.getEnvVarExist("OAUTH2_PROXY_REDIS_CONNECTION_URL", actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_CONNECTION_URL should not be present when SessionStoreType is cookie")

	// Env var OAUTH2_PROXY_REDIS_PASSWORD and OAUTH2_PROXY_REDIS_CONNECTION_URL should be present again when SessionStoreType is systemManaged redis
	returnOAuth.SessionStoreType = radixv1.SessionStoreSystemManaged
	s.oauth2Config.EXPECT().MergeWith(inputOAuth).Times(1).Return(returnOAuth, nil)
	s.Require().NoError(sut.Sync(context.Background()))

	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, 31, "Unexpected amount of env-vars")
	s.True(s.getEnvVarExist(oauth2ProxyRedisPasswordEnvironmentVariable, actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie")
	redisConnectUrlEnvVar, redisConnectUrlExists := slice.FindFirst(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool {
		return ev.Name == "OAUTH2_PROXY_REDIS_CONNECTION_URL"
	})
	s.True(redisConnectUrlExists, "Env var OAUTH2_PROXY_REDIS_CONNECTION_URL should be present when SessionStoreType is systemManaged")
	s.Equal("redis://server-aux-oauth-redis:6379", redisConnectUrlEnvVar.Value, "Invalid env var OAUTH2_PROXY_REDIS_CONNECTION_URL")
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxy_SecretCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{}, nil)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})).
		BuildRD()
	sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(expectedSecretName, actualSecrets.Items[0].Name)
	s.Equal(expectedLabels, actualSecrets.Items[0].Labels)
	s.NotEmpty(actualSecrets.Items[0].Data[defaults.OAuthCookieSecretKeyName])
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxy_RbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{}, nil)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}
	readerGroups, readerUsers := []string{"rdr1", "rdr2"}, []string{"rdrUsr1", "rdrUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).WithReaderAdGroups(readerGroups).WithReaderAdUsers(readerUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})).
		BuildRD()
	sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))

	expectedRoles := []string{fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)), fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix))}
	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)

	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleNames(actualRoles))

	admRole := getRoleByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, admRole.Labels)
	s.Len(admRole.Rules, 1)
	s.ElementsMatch([]string{""}, admRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, admRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, admRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "update", "patch", "list", "watch", "delete"}, admRole.Rules[0].Verbs)

	readerRole := getRoleByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, readerRole.Labels)
	s.Len(readerRole.Rules, 1)
	s.ElementsMatch([]string{""}, readerRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, readerRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, readerRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "list", "watch"}, readerRole.Rules[0].Verbs)

	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleBindingNames(actualRoleBindings))

	admRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)), actualRoleBindings)
	s.Equal(expectedLabels, admRoleBinding.Labels)
	s.Equal(admRole.Name, admRoleBinding.RoleRef.Name)
	expectedSubjects := []rbacv1.Subject{
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm1"},
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm2"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr1"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr2"},
	}
	s.ElementsMatch(expectedSubjects, admRoleBinding.Subjects)

	readerRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)), actualRoleBindings)
	s.Equal(expectedLabels, readerRoleBinding.Labels)
	s.Equal(readerRole.Name, readerRoleBinding.RoleRef.Name)
	expectedSubjects = []rbacv1.Subject{
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "rdr1"},
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "rdr2"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "rdrUsr1"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "rdrUsr2"},
	}
	s.ElementsMatch(expectedSubjects, readerRoleBinding.Subjects)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxySecret_SecretKeysGarbageCollected() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SessionStoreType: radixv1.SessionStoreRedis}, nil)

	secretName := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
	_, err := s.kubeClient.CoreV1().Secrets(envNs).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName},
			Data: map[string][]byte{
				defaults.OAuthRedisPasswordKeyName: []byte("redispassword"),
				defaults.OAuthCookieSecretKeyName:  []byte("cookiesecret"),
				defaults.OAuthClientSecretKeyName:  []byte("clientsecret"),
			},
		},
		metav1.CreateOptions{})
	s.Require().NoError(err)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})).
		BuildRD()
	sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))

	// Keep redispassword if sessionstoretype is redis
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	s.Equal(
		map[string][]byte{
			defaults.OAuthRedisPasswordKeyName: []byte("redispassword"),
			defaults.OAuthCookieSecretKeyName:  []byte("cookiesecret"),
			defaults.OAuthClientSecretKeyName:  []byte("clientsecret")},
		actualSecret.Data)

	// Remove redispassword if sessionstoretype is cookie
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SessionStoreType: radixv1.SessionStoreCookie}, nil)
	err = sut.Sync(context.Background())
	s.Require().NoError(err)
	actualSecret, _ = s.kubeClient.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	s.Equal(
		map[string][]byte{
			defaults.OAuthCookieSecretKeyName: []byte("cookiesecret"),
			defaults.OAuthClientSecretKeyName: []byte("clientsecret")},
		actualSecret.Data)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxy_ServiceCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{}, nil)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})).
		BuildRD()
	sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}
	expectedServiceName := utils.GetAuxOAuthProxyComponentServiceName(componentName)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(expectedServiceName, actualServices.Items[0].Name)
	s.Equal(expectedLabels, actualServices.Items[0].Labels)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualServices.Items[0].OwnerReferences)
	s.Equal(corev1.ServiceTypeClusterIP, actualServices.Items[0].Spec.Type)
	s.Len(actualServices.Items[0].Spec.Ports, 1)
	s.Equal(corev1.ServicePort{Port: defaults.OAuthProxyPortNumber, TargetPort: intstr.FromString(defaults.OAuthProxyPortName), Protocol: corev1.ProtocolTCP}, actualServices.Items[0].Spec.Ports[0])
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxy_IngressesCreated() {
	const (
		appName   = "anyapp"
		envName   = "anyenv"
		comp1Name = "comp1"
		comp2Name = "comp2"
	)

	stdAnnotations1 := map[string]string{"foo1-1": "bar1", "foo1-2": "bar2"}
	stdAnnotationProvider1 := ingress.NewMockAnnotationProvider(s.ctrl)
	stdAnnotationProvider1.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(stdAnnotations1, nil)
	stdAnnotations2 := map[string]string{"foo2-1": "baz1", "foo2-2": "baz2"}
	stdAnnotationProvider2 := ingress.NewMockAnnotationProvider(s.ctrl)
	stdAnnotationProvider2.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(stdAnnotations2, nil)
	ingressAnnotationProviders := []ingress.AnnotationProvider{stdAnnotationProvider1, stdAnnotationProvider2}
	proxyAnnotations := map[string]string{"proxy1": "bar1", "proxy2": "bar2"}
	proxyAnnotatioProvider := ingress.NewMockAnnotationProvider(s.ctrl)
	proxyAnnotatioProvider.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(proxyAnnotations, nil)
	proxyAnnotationProviders := []ingress.AnnotationProvider{proxyAnnotatioProvider}

	tests := map[string]struct {
		rrAnnotations       map[string]string
		rdAnnotations       map[string]string
		comp1ExpectedPath   string
		comp2ExpectedPath   string
		expectedAnnotations map[string]string
	}{
		"proxy mode disabled": {
			rdAnnotations:       nil,
			rrAnnotations:       nil,
			comp1ExpectedPath:   "/oauth2",
			comp2ExpectedPath:   "/custompath",
			expectedAnnotations: annotations.Merge(stdAnnotations1, stdAnnotations2),
		},
		"proxy mode enabled by RD": {
			rdAnnotations:       map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			rrAnnotations:       nil,
			comp1ExpectedPath:   "/",
			comp2ExpectedPath:   "/",
			expectedAnnotations: proxyAnnotations,
		},
		"proxy mode enabled by RR": {
			rdAnnotations:       nil,
			rrAnnotations:       map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			comp1ExpectedPath:   "/",
			comp2ExpectedPath:   "/",
			expectedAnnotations: proxyAnnotations,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			envNamespace := utils.GetEnvironmentNamespace(appName, envName)
			appAliasLabel := map[string]string{kube.RadixAppAliasLabel: "true"}
			externalAliasLabel := map[string]string{kube.RadixExternalAliasLabel: "true"}
			clusterNameAliasLabel := map[string]string{kube.RadixDefaultAliasLabel: "true"}
			activeClusterAliasLabel := map[string]string{kube.RadixActiveClusterAliasLabel: "true"}
			comp1BaseLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: comp1Name, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}
			comp2BaseLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: comp2Name, kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType}

			rr := utils.NewRegistrationBuilder().WithName(appName).WithAnnotations(test.rrAnnotations).BuildRR()
			rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithAnnotations(test.rdAnnotations).
				WithComponents(
					utils.NewDeployComponentBuilder().WithName(comp1Name).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}).
						WithDNSAppAlias(true).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo1.bar.com"}),
					utils.NewDeployComponentBuilder().WithName(comp2Name).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ProxyPrefix: "/custompath"}}).
						WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo2.bar.com"}),
				).
				BuildRD()

			sut := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()), ingressAnnotationProviders, proxyAnnotationProviders, "", "")
			s.Require().NoError(sut.Sync(context.Background()))

			actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			expectedNames := []string{
				"anyapp-url-alias-aux-oauth", "comp1-active-cluster-url-alias-aux-oauth", "comp1-aux-oauth", "foo1.bar.com-aux-oauth",
				"comp2-active-cluster-url-alias-aux-oauth", "comp2-aux-oauth", "foo2.bar.com-aux-oauth",
			}
			actualNames := slice.Map(actualIngresses.Items, func(o networkingv1.Ingress) string { return o.Name })
			s.Require().ElementsMatch(expectedNames, actualNames)

			buildExpectedIngressSpec := func(host, tlsSecret, path, component string) networkingv1.IngressSpec {
				return networkingv1.IngressSpec{
					IngressClassName: pointers.Ptr("nginx"),
					TLS: []networkingv1.IngressTLS{{
						Hosts: []string{host}, SecretName: tlsSecret}},
					Rules: []networkingv1.IngressRule{{
						Host: host,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									PathType: pointers.Ptr(networkingv1.PathTypeImplementationSpecific),
									Path:     path,
									Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
										Name: utils.GetAuxOAuthProxyComponentServiceName(component),
										Port: networkingv1.ServiceBackendPort{Number: defaults.OAuthProxyPortNumber},
									}}}}}}}},
				}
			}

			// comp1: anyapp-url-alias-aux-oauth
			actualIng := getIngressByName("anyapp-url-alias-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp1BaseLabels, appAliasLabel), actualIng.Labels)
			expectedSpec := buildExpectedIngressSpec(appName+"."+s.appAliasDnsZone, defaults.TLSSecretName, test.comp1ExpectedPath, comp1Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp1: anyapp-url-alias-aux-oauth
			actualIng = getIngressByName("comp1-active-cluster-url-alias-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp1BaseLabels, activeClusterAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec(comp1Name+"-"+envNamespace+"."+s.dnsZone, defaults.TLSSecretName, test.comp1ExpectedPath, comp1Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp1: anyapp-url-alias-aux-oauth
			actualIng = getIngressByName("comp1-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp1BaseLabels, clusterNameAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec(comp1Name+"-"+envNamespace+"."+s.clusterName+"."+s.dnsZone, defaults.TLSSecretName, test.comp1ExpectedPath, comp1Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp1: anyapp-url-alias-aux-oauth
			actualIng = getIngressByName("foo1.bar.com-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp1BaseLabels, externalAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec("foo1.bar.com", "foo1.bar.com", test.comp1ExpectedPath, comp1Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp2: comp2-active-cluster-url-alias-aux-oauth
			actualIng = getIngressByName("comp2-active-cluster-url-alias-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp2BaseLabels, activeClusterAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec(comp2Name+"-"+envNamespace+"."+s.dnsZone, defaults.TLSSecretName, test.comp2ExpectedPath, comp2Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp2: comp2-aux-oauth
			actualIng = getIngressByName("comp2-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp2BaseLabels, clusterNameAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec(comp2Name+"-"+envNamespace+"."+s.clusterName+"."+s.dnsZone, defaults.TLSSecretName, test.comp2ExpectedPath, comp2Name)
			s.Equal(expectedSpec, actualIng.Spec)

			// comp2: foo2.bar.com-aux-oauth
			actualIng = getIngressByName("foo2.bar.com-aux-oauth", actualIngresses)
			s.Equal(test.expectedAnnotations, actualIng.Annotations)
			s.Equal(maps.MergeMaps(comp2BaseLabels, externalAliasLabel), actualIng.Labels)
			expectedSpec = buildExpectedIngressSpec("foo2.bar.com", "foo2.bar.com", test.comp2ExpectedPath, comp2Name)
			s.Equal(expectedSpec, actualIng.Spec)
		})
	}

}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetIngressName() {
	actualIngressName := oauthutil.GetAuxOAuthProxyIngressName("ing")
	s.Equal(fmt.Sprintf("%s-%s", "ing", radixv1.OAuthProxyAuxiliaryComponentSuffix), actualIngressName)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect_ComponentRemoved() {
	const (
		appName = "anyapp"
		envName = "dev"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)

	// Fixture
	rr := utils.NewRegistrationBuilder().WithName(appName)
	comp1 := utils.NewDeployComponentBuilder().WithName("c1").WithPort("http", 8000).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})
	comp2 := utils.NewDeployComponentBuilder().WithName("c2").WithPort("http", 8000).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})
	rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp1, comp2)
	sut := NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))
	actualDeployments, _ := s.kubeClient.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualDeployments.Items, 2)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualSecrets.Items, 2)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualServices.Items, 2)
	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualIngresses.Items, 4)
	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualRoles.Items, 4)
	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualRoleBindings.Items, 4)

	// Test garbage collect
	rd = utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp1)
	sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.GarbageCollect(context.Background()))
	actualDeployments, _ = s.kubeClient.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeployments.Items, 1)
	actualSecrets, _ = s.kubeClient.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	actualServices, _ = s.kubeClient.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 2)
	actualRoles, _ = s.kubeClient.RbacV1().Roles(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 2)
	actualRoleBindings, _ = s.kubeClient.RbacV1().RoleBindings(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 2)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect_OAuthDisabled_RemovedIngresses() {
	const (
		appName = "anyapp"
		envName = "dev"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)

	// Fixture
	rr := utils.NewRegistrationBuilder().WithName(appName)
	comp := utils.NewDeployComponentBuilder().WithName("c1").WithPort("http", 8000).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})
	rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut := NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))
	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualIngresses.Items, 2)

	// Test garbage collect
	comp.WithAuthentication(nil)
	rd = utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.GarbageCollect(context.Background()))
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 0)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect_IngressConfigRemoved() {
	const (
		appName = "anyapp"
		envName = "dev"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{ProxyPrefix: "/oauth2"}, nil)

	// Fixture
	rr := utils.NewRegistrationBuilder().WithName(appName)
	comp := utils.NewDeployComponentBuilder().WithName("c1").WithPort("http", 8000).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}).
		WithDNSAppAlias(true).
		WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo1.example.com"}, radixv1.RadixDeployExternalDNS{FQDN: "foo2.example.com"})
	rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut := NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.Sync(context.Background()))
	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	actualNames := slice.Map(actualIngresses.Items, func(o networkingv1.Ingress) string { return o.Name })
	s.ElementsMatch([]string{"anyapp-url-alias-aux-oauth", "c1-active-cluster-url-alias-aux-oauth", "c1-aux-oauth", "foo1.example.com-aux-oauth", "foo2.example.com-aux-oauth"}, actualNames)

	// Garbage collect no changes
	rd = utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.GarbageCollect(context.Background()))
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	actualNames = slice.Map(actualIngresses.Items, func(o networkingv1.Ingress) string { return o.Name })
	s.ElementsMatch([]string{"anyapp-url-alias-aux-oauth", "c1-active-cluster-url-alias-aux-oauth", "c1-aux-oauth", "foo1.example.com-aux-oauth", "foo2.example.com-aux-oauth"}, actualNames)

	// Garbage collect - disable app alias
	comp = comp.WithDNSAppAlias(false)
	rd = utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.GarbageCollect(context.Background()))
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	actualNames = slice.Map(actualIngresses.Items, func(o networkingv1.Ingress) string { return o.Name })
	s.ElementsMatch([]string{"c1-active-cluster-url-alias-aux-oauth", "c1-aux-oauth", "foo1.example.com-aux-oauth", "foo2.example.com-aux-oauth"}, actualNames)

	// Garbage collect - remove an external dns alias
	comp = comp.WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo2.example.com"})
	rd = utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponents(comp)
	sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
	s.Require().NoError(sut.GarbageCollect(context.Background()))
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	actualNames = slice.Map(actualIngresses.Items, func(o networkingv1.Ingress) string { return o.Name })
	s.ElementsMatch([]string{"c1-active-cluster-url-alias-aux-oauth", "c1-aux-oauth", "foo2.example.com-aux-oauth"}, actualNames)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect_SwitchProxyMode() {
	const (
		appName = "anyapp"
		envName = "dev"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	tests := map[string]struct {
		fixtureRDAnnotations   map[string]string
		fixtureRRAnnotations   map[string]string
		testRDAnnotations      map[string]string
		testRRAnnotations      map[string]string
		expectIngressesDeleted bool
	}{
		"non-proxy on fixture and test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      nil,
			testRRAnnotations:      nil,
			expectIngressesDeleted: false,
		},
		"non-proxy on fixture, rd proxy on test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRRAnnotations:      nil,
			expectIngressesDeleted: true,
		},
		"non-proxy on fixture, rr proxy on test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      nil,
			testRRAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			expectIngressesDeleted: true,
		},
		"rd proxy on fixture, non-proxy on test": {
			fixtureRDAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      nil,
			testRRAnnotations:      nil,
			expectIngressesDeleted: true,
		},
		"rd proxy on fixture, rd proxy on test": {
			fixtureRDAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRRAnnotations:      nil,
			expectIngressesDeleted: false,
		},
		"rd proxy on fixture, rr proxy on test": {
			fixtureRDAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			fixtureRRAnnotations:   nil,
			testRDAnnotations:      nil,
			testRRAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			expectIngressesDeleted: false,
		},
		"rr proxy on fixture, non-proxy on test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRDAnnotations:      nil,
			testRRAnnotations:      nil,
			expectIngressesDeleted: true,
		},
		"rr proxy on fixture, rd proxy on test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRDAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRRAnnotations:      nil,
			expectIngressesDeleted: false,
		},
		"rr proxy on fixture, rr proxy on test": {
			fixtureRDAnnotations:   nil,
			fixtureRRAnnotations:   map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			testRDAnnotations:      nil,
			testRRAnnotations:      map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "*"},
			expectIngressesDeleted: false,
		},
	}

	for name, test := range tests {
		s.Run(name, func() {
			s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{ProxyPrefix: "/oauth2"}, nil)

			// Fixture
			rr := utils.NewRegistrationBuilder().WithName(appName).WithAnnotations(test.fixtureRRAnnotations)
			comp := utils.NewDeployComponentBuilder().WithName("c1").WithPort("http", 8000).WithPublicPort("http").WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}).
				WithDNSAppAlias(true).
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "foo1.example.com"}, radixv1.RadixDeployExternalDNS{FQDN: "foo2.example.com"})
			rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithAnnotations(test.fixtureRDAnnotations).WithComponents(comp)
			sut := NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.Sync(context.Background()))
			actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NotEmpty(actualIngresses.Items)

			// Test
			rr = rr.WithAnnotations(test.testRRAnnotations)
			rd = rd.WithAnnotations(test.testRDAnnotations)
			sut = NewOAuthProxyResourceManager(rd.BuildRD(), rr.BuildRR(), s.kubeUtil, s.oauth2Config, nil, nil, "", "")
			s.Require().NoError(sut.GarbageCollect(context.Background()))
			actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			if test.expectIngressesDeleted {
				s.Empty(actualIngresses.Items)
			} else {
				s.NotEmpty(actualIngresses.Items)
			}
		})
	}
}

func (*OAuthProxyResourceManagerTestSuite) getEnvVarExist(name string, envvars []corev1.EnvVar) bool {
	for _, envvar := range envvars {
		if envvar.Name == name {
			return true
		}
	}
	return false
}

func (*OAuthProxyResourceManagerTestSuite) getEnvVarValueByName(name string, envvars []corev1.EnvVar) string {
	for _, envvar := range envvars {
		if envvar.Name == name {
			return envvar.Value
		}
	}
	return ""
}

func (*OAuthProxyResourceManagerTestSuite) getEnvVarValueFromByName(name string, envvars []corev1.EnvVar) corev1.EnvVarSource {
	for _, envvar := range envvars {
		if envvar.Name == name && envvar.ValueFrom != nil {
			return *envvar.ValueFrom
		}
	}
	return corev1.EnvVarSource{}
}
