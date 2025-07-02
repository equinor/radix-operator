package deployment

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
}

func TestOAuthProxyResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthProxyResourceManagerTestSuite))
}

func (s *OAuthProxyResourceManagerTestSuite) SetupSuite() {
	s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, "1234-5678-91011")
	s.T().Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, "app.dev.radix.equinor.com")
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	s.T().Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	s.T().Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	s.T().Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler:main-latest")
	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
	s.T().Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, "oidc_issuer_url")
}

func (s *OAuthProxyResourceManagerTestSuite) SetupTest() {
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
	ingressAnnotationProviders := []ingress.AnnotationProvider{&ingress.MockAnnotationProvider{}}

	oauthManager := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil, oauthConfig, ingressAnnotationProviders, "proxy:123")
	sut, ok := oauthManager.(*oauthProxyResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
	s.Equal(oauthConfig, sut.oauth2DefaultConfig)
	s.Equal(ingressAnnotationProviders, sut.ingressAnnotationProviders)
	s.Equal("proxy:123", sut.oauth2ProxyDockerImage)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_ComponentRestartEnvVar() {
	auth := &v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234"}}
	appName := "anyapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}
	type testSpec struct {
		name                string
		rd                  *v1.RadixDeployment
		expectRestartEnvVar bool
	}
	tests := []testSpec{
		{
			name: "component default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithEnvironmentVariable(defaults.RadixRestartEnvironmentVariable, "anyval")).
				BuildRD(),
			expectRestartEnvVar: true,
		},
		{
			name: "component replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectRestartEnvVar: false,
		},
	}
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&v1.OAuth2{}, nil)
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthProxyResourceManager{test.rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			envVarExist := slice.Any(deploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool { return ev.Name == defaults.RadixRestartEnvironmentVariable })
			s.Equal(test.expectRestartEnvVar, envVarExist)
		})
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_NotPublicOrNoOAuth() {
	appName := "anyapp"
	type scenarioDef struct{ rd *v1.RadixDeployment }
	scenarios := []scenarioDef{
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithPublicPort("http")).BuildRD()},
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234"}})).BuildRD()},
	}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for _, scenario := range scenarios {
		sut := &oauthProxyResourceManager{scenario.rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
		err := sut.Sync(context.Background())
		s.Nil(err)
		deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(deploys.Items, 0)
		services, _ := s.kubeClient.CoreV1().Services(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(services.Items, 0)
		ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(ingresses.Items, 0)
		secrets, _ := s.kubeClient.CoreV1().Secrets(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(secrets.Items, 0)
		roles, _ := s.kubeClient.RbacV1().Roles(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(roles.Items, 0)
		rolebindings, _ := s.kubeClient.RbacV1().RoleBindings(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(rolebindings.Items, 0)
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_UseClientSecretOrIdentity() {
	type scenarioDef struct {
		name                        string
		rd                          *v1.RadixDeployment
		expectedSa                  *corev1.ServiceAccount
		expectedAuxOauthDeployCount int
		existingSa                  *corev1.ServiceAccount
	}
	const (
		appName                    = "anyapp"
		componentAzureClientId     = "some-azure-client-id"
		oauth2ClientId             = "oauth2-client-id"
		componentName1             = "comp1"
		expectedServiceAccountName = "comp1-aux-oauth-sa"
		envName                    = "qa"
	)

	identity := &v1.Identity{Azure: &v1.AzureIdentity{ClientId: componentAzureClientId}}
	auxOAuthServiceAccount := buildServiceAccount(utils.GetEnvironmentNamespace(appName, envName), expectedServiceAccountName,
		getLabelsForAuxOAuthComponentServiceAccount(componentName1),
		oauth2ClientId)
	scenarios := []scenarioDef{
		{name: "Not created service account without Authentication", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: componentAzureClientId}})).
			BuildRD(),
			expectedAuxOauthDeployCount: 0},
		{name: "Not created service account with Authentication, no OAuth2", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: componentAzureClientId}}).
			WithAuthentication(&v1.Authentication{})).BuildRD(),
			expectedAuxOauthDeployCount: 0},
		{name: "Not created service account with Authentication, OAuth2, false useAzureIdentity", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: componentAzureClientId}}).
			WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: oauth2ClientId}})).BuildRD(),
			expectedAuxOauthDeployCount: 1,
		},
		{name: "Created the service account with Authentication, OAuth2, true useAzureIdentity", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
			WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(identity).
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: oauth2ClientId, Credentials: v1.AzureWorkloadIdentity}})).BuildRD(),
			expectedAuxOauthDeployCount: 1,
			expectedSa:                  auxOAuthServiceAccount,
		},
		{name: "Delete existing service account, with Authentication, OAuth2, no useAzureIdentity", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: componentAzureClientId}}).
			WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: oauth2ClientId}})).BuildRD(),
			expectedAuxOauthDeployCount: 1,
			existingSa:                  auxOAuthServiceAccount,
		},
		{name: "Not overridden the existing service account with Authentication, OAuth2, true useAzureIdentity", rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment(envName).
			WithComponent(utils.NewDeployComponentBuilder().WithName(componentName1).WithPublicPort("http").WithIdentity(identity).
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: oauth2ClientId, Credentials: v1.AzureWorkloadIdentity}})).BuildRD(),
			expectedAuxOauthDeployCount: 1,
			existingSa:                  auxOAuthServiceAccount,
			expectedSa:                  auxOAuthServiceAccount,
		},
	}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.setupTest()
			sut := &oauthProxyResourceManager{scenario.rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()), "", zerolog.Nop()}
			if scenario.existingSa != nil {
				_, err := sut.kubeutil.KubeClient().CoreV1().ServiceAccounts(scenario.existingSa.Namespace).Create(context.Background(), scenario.existingSa, metav1.CreateOptions{})
				s.NoError(err, "Failed to create service account")
			}
			auxOAuthSecret, err := sut.buildOAuthProxySecret(appName, &scenario.rd.Spec.Components[0])
			auxOAuthSecret.SetNamespace(scenario.rd.Namespace)
			s.NoError(err, "Failed to build secret")
			auxOAuthSecret.Data[defaults.OAuthClientSecretKeyName] = []byte("some-client-secret")
			_, err = s.kubeUtil.KubeClient().CoreV1().Secrets(scenario.rd.Namespace).Create(context.Background(), auxOAuthSecret, metav1.CreateOptions{})
			s.NoError(err, "Failed to create secret")

			err = sut.Sync(context.Background())
			s.Nil(err)

			deploys, err := sut.kubeutil.KubeClient().AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			s.NoError(err, "Failed to list deployments")
			s.Len(deploys.Items, scenario.expectedAuxOauthDeployCount, "Expected aux oauth deployment count")
			saList, err := sut.kubeutil.KubeClient().CoreV1().ServiceAccounts(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: getLabelsForAuxOAuthComponentServiceAccount(componentName1).String()})
			s.NoError(err, "Failed to list service accounts")
			existsClientSecretEnvVar := false
			existsOAuthProxyDeployment := len(deploys.Items) > 0
			if existsOAuthProxyDeployment {
				auxOAuthDeployment := deploys.Items[0]
				auxOAuthContainer := auxOAuthDeployment.Spec.Template.Spec.Containers[0]
				secretName := utils.GetAuxiliaryComponentSecretName(scenario.rd.Spec.Components[0].GetName(), v1.OAuthProxyAuxiliaryComponentSuffix)
				existsClientSecretEnvVar = slice.Any(auxOAuthContainer.Env, func(ev corev1.EnvVar) bool {
					return ev.Name == oauth2ProxyClientSecretEnvironmentVariable && ev.ValueFrom != nil && ev.ValueFrom.SecretKeyRef != nil &&
						ev.ValueFrom.SecretKeyRef.Key == defaults.OAuthClientSecretKeyName &&
						ev.ValueFrom.SecretKeyRef.LocalObjectReference.Name == secretName
				})
				actualSecret, err := sut.kubeutil.KubeClient().CoreV1().Secrets(scenario.rd.Namespace).Get(context.Background(), auxOAuthSecret.GetName(), metav1.GetOptions{})
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

func getLabelsForAuxOAuthComponentServiceAccount(componentName string) labels.Set {
	return radixlabels.ForOAuthProxyComponentServiceAccount(&v1.RadixDeployComponent{Name: componentName})
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OauthDeploymentReplicas() {
	auth := &v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234"}}
	appName := "anyapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}
	type testSpec struct {
		name             string
		rd               *v1.RadixDeployment
		expectedReplicas int32
	}
	tests := []testSpec{
		{
			name: "component default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp()).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		{
			name: "component replicas set override to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		{
			name: "component with hpa and default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component with hpa and replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component with hpa and replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{

			name: "component with hpa and replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 0,
		},
	}
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&v1.OAuth2{}, nil)
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthProxyResourceManager{test.rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := sut.kubeutil.KubeClient().AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			s.Equal(test.expectedReplicas, *deploys.Items[0].Spec.Replicas)
		})
	}
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyDeploymentCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	inputOAuth := &v1.OAuth2{ClientID: "1234"}
	returnOAuth := &v1.OAuth2{
		ClientID:               commonUtils.RandString(20),
		Scope:                  commonUtils.RandString(20),
		SetXAuthRequestHeaders: pointers.Ptr(true),
		SetAuthorizationHeader: pointers.Ptr(false),
		ProxyPrefix:            commonUtils.RandString(20),
		LoginURL:               commonUtils.RandString(20),
		RedeemURL:              commonUtils.RandString(20),
		SessionStoreType:       "redis",
		Cookie: &v1.OAuth2Cookie{
			Name:     commonUtils.RandString(20),
			Expire:   commonUtils.RandString(20),
			Refresh:  commonUtils.RandString(20),
			SameSite: v1.CookieSameSiteType(commonUtils.RandString(20)),
		},
		CookieStore: &v1.OAuth2CookieStore{
			Minimal: pointers.Ptr(true),
		},
		RedisStore: &v1.OAuth2RedisStore{
			ConnectionURL: commonUtils.RandString(20),
		},
		OIDC: &v1.OAuth2OIDC{
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
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: inputOAuth}).WithRuntime(&v1.Runtime{Architecture: "customarch"})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)

	actualDeploy := actualDeploys.Items[0]
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix), actualDeploy.Name)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualDeploy.OwnerReferences)

	expectedDeployLabels := map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType,
	}
	expectedPodLabels := map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAppIDLabel:                  rr.Spec.AppID.String(),
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType,
	}
	s.Equal(expectedDeployLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedPodLabels, actualDeploy.Spec.Template.Labels)

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(sut.oauth2ProxyDockerImage, defaultContainer.Image)

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
	secretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthCookieSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyCookieSecretEnvironmentVariable, defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthClientSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyClientSecretEnvironmentVariable, defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthRedisPasswordKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyRedisPasswordEnvironmentVariable, defaultContainer.Env))

	// Env var OAUTH2_PROXY_REDIS_PASSWORD and OAUTH2_PROXY_REDIS_CONNECTION_URL should not be present when SessionStoreType is cookie
	returnOAuth.SessionStoreType = "cookie"
	s.oauth2Config.EXPECT().MergeWith(inputOAuth).Times(1).Return(returnOAuth, nil)
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, 29, "Unexpected amount of env-vars")
	s.False(s.getEnvVarExist(oauth2ProxyRedisPasswordEnvironmentVariable, actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie")
	s.False(s.getEnvVarExist("OAUTH2_PROXY_REDIS_CONNECTION_URL", actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_CONNECTION_URL should not be present when SessionStoreType is cookie")

	// Env var OAUTH2_PROXY_REDIS_PASSWORD and OAUTH2_PROXY_REDIS_CONNECTION_URL should be present again when SessionStoreType is systemManaged redis
	returnOAuth.SessionStoreType = v1.SessionStoreSystemManaged
	s.oauth2Config.EXPECT().MergeWith(inputOAuth).Times(1).Return(returnOAuth, nil)
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, 31, "Unexpected amount of env-vars")
	s.True(s.getEnvVarExist(oauth2ProxyRedisPasswordEnvironmentVariable, actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env), "Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie")
	redisConnectUrlEnvVar, redisConnectUrlExists := slice.FindFirst(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool {
		return ev.Name == "OAUTH2_PROXY_REDIS_CONNECTION_URL"
	})
	s.True(redisConnectUrlExists, "Env var OAUTH2_PROXY_REDIS_CONNECTION_URL should be present when SessionStoreType is systemManaged")
	s.Equal("redis://server-aux-oauth-redis:6379", redisConnectUrlEnvVar.Value, "Invalid env var OAUTH2_PROXY_REDIS_CONNECTION_URL")
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxySecretAndRbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{}, nil)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}
	// readerGroups, readerUsers := []string{"rdr1", "rdr2"}, []string{"rdrUsr1", "rdrUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(expectedSecretName, actualSecrets.Items[0].Name)
	s.Equal(expectedLabels, actualSecrets.Items[0].Labels)
	s.NotEmpty(actualSecrets.Items[0].Data[defaults.OAuthCookieSecretKeyName])
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyRbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{}, nil)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}
	readerGroups, readerUsers := []string{"rdr1", "rdr2"}, []string{"rdrUsr1", "rdrUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).WithReaderAdGroups(readerGroups).WithReaderAdUsers(readerUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedRoles := []string{fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)), fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix))}
	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)

	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleNames(actualRoles))

	admRole := getRoleByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, admRole.Labels)
	s.Len(admRole.Rules, 1)
	s.ElementsMatch([]string{""}, admRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, admRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, admRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "update", "patch", "list", "watch", "delete"}, admRole.Rules[0].Verbs)

	readerRole := getRoleByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, readerRole.Labels)
	s.Len(readerRole.Rules, 1)
	s.ElementsMatch([]string{""}, readerRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, readerRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, readerRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "list", "watch"}, readerRole.Rules[0].Verbs)

	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleBindingNames(actualRoleBindings))

	admRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)), actualRoleBindings)
	s.Equal(expectedLabels, admRoleBinding.Labels)
	s.Equal(admRole.Name, admRoleBinding.RoleRef.Name)
	expectedSubjects := []rbacv1.Subject{
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm1"},
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm2"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr1"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr2"},
	}
	s.ElementsMatch(expectedSubjects, admRoleBinding.Subjects)

	readerRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)), actualRoleBindings)
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

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxySecret_KeysGarbageCollected() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}, nil)

	secretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
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
	s.Nil(err)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.Nil(err)

	// Keep redispassword if sessionstoretype is redis
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	s.Equal(
		map[string][]byte{
			defaults.OAuthRedisPasswordKeyName: []byte("redispassword"),
			defaults.OAuthCookieSecretKeyName:  []byte("cookiesecret"),
			defaults.OAuthClientSecretKeyName:  []byte("clientsecret")},
		actualSecret.Data)

	// Remove redispassword if sessionstoretype is cookie
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{SessionStoreType: v1.SessionStoreCookie}, nil)
	err = sut.Sync(context.Background())
	s.Require().NoError(err)
	actualSecret, _ = s.kubeClient.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	s.Equal(
		map[string][]byte{
			defaults.OAuthCookieSecretKeyName: []byte("cookiesecret"),
			defaults.OAuthClientSecretKeyName: []byte("clientsecret")},
		actualSecret.Data)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyServiceCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{}, nil)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType}
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

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyIngressesCreated() {
	appName, envName, component1Name, component2Name := "anyapp", "qa", "server", "web"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	ingServer := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing1", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component1Name, kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingServerNoRules := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing2", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component1Name, kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
		},
	}
	ingOtherComponent := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing3", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: "othercomponent", kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingServerOtherNs := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing4", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component1Name, kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingWeb1 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing5", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass1"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing2-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing2.local"}},
		},
	}
	ingWeb2 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing6", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixDefaultAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass2"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing2-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing2.public"}},
		},
	}
	ingWeb3 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ingExtAlias", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixExternalAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass3"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing3-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing3.public"}},
		},
	}
	ingWeb4 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ingActiveClusterAlias", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixActiveClusterAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass4"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing4-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing4.public"}},
		},
	}
	ingWeb5 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ingAppAlias", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixAppAliasLabel: "true"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass5"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing5-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing5.public"}},
		},
	}
	_, err := s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingServer, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingServerNoRules, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses("otherns").Create(context.Background(), &ingServerOtherNs, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingOtherComponent, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb1, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb2, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb3, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb4, metav1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb5, metav1.CreateOptions{})
	s.Require().NoError(err)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWith(rd.Spec.Components[0].Authentication.OAuth2).Times(1).Return(&v1.OAuth2{ProxyPrefix: "auth1"}, nil)
	s.oauth2Config.EXPECT().MergeWith(rd.Spec.Components[1].Authentication.OAuth2).Times(1).Return(&v1.OAuth2{ProxyPrefix: "auth2"}, nil)

	expectedIngServerCall := rd.Spec.Components[0].DeepCopy()
	expectedIngServerCall.Authentication.OAuth2.ProxyPrefix = "auth1"
	expectedIngServerAnnotations := map[string]string{"annotation1-1": "val1-1", "annotation1-2": "val1-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngServerCall, rd.Namespace).Times(1).Return(expectedIngServerAnnotations, nil)
	expectedIngWebCall := rd.Spec.Components[1].DeepCopy()
	expectedIngWebCall.Authentication.OAuth2.ProxyPrefix = "auth2"
	expectedIngWebAnnotations := map[string]string{"annotation2-1": "val2-1", "annotation2-2": "val2-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngWebCall, rd.Namespace).Times(5).Return(expectedIngWebAnnotations, nil)
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{s.ingressAnnotationProvider}, s.oauth2Config, "", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.NoError(err)

	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 15)
	getIngress := func(name string, ingresses []networkingv1.Ingress) *networkingv1.Ingress {
		for _, ing := range ingresses {
			if ing.Name == name {
				return &ing
			}
		}
		return nil
	}
	getExpectedIngressRule := func(componentName, proxyPrefix string) networkingv1.IngressRuleValue {
		pathType := networkingv1.PathTypeImplementationSpecific
		return networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     oauthutil.SanitizePathPrefix(proxyPrefix),
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: utils.GetAuxOAuthProxyComponentServiceName(componentName),
							Port: networkingv1.ServiceBackendPort{Number: defaults.OAuthProxyPortNumber},
						},
					},
				}},
			},
		}
	}
	// Ingresses for server component
	ingressName := fmt.Sprintf("%s-%s", ingServer.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress := getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngServerAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingServer), actualIngress.OwnerReferences)
	s.Equal(ingServer.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingServer.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingServer.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component1Name, "auth1"), actualIngress.Spec.Rules[0].IngressRuleValue)

	// Ingresses for web component
	ingressName = fmt.Sprintf("%s-%s", ingWeb1.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb1), actualIngress.OwnerReferences)
	s.Equal(ingWeb1.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb1.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb1.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb2.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb2), actualIngress.OwnerReferences)
	s.Equal(ingWeb2.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb2.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb2.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb3.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb3), actualIngress.OwnerReferences)
	s.Equal(ingWeb3.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb3.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb3.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb4.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb4), actualIngress.OwnerReferences)
	s.Equal(ingWeb4.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb4.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb4.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb5.Name, v1.OAuthProxyAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb5), actualIngress.OwnerReferences)
	s.Equal(ingWeb5.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb5.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb5.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyUninstall() {
	appName, envName, component1Name, component2Name := "anyapp", "qa", "server", "web"
	envNs := utils.GetEnvironmentNamespace(appName, envName)

	_, err := s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing1", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component1Name, kube.RadixDefaultAliasLabel: "true"}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing2", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixDefaultAliasLabel: "true"}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)
	s.Require().NoError(err)
	_, err = s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing3", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name, kube.RadixDefaultAliasLabel: "true"}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)
	s.Require().NoError(err)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(2).Return(&v1.OAuth2{}, nil)
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.NoError(err)

	// Bootstrap oauth proxy resources for two components
	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 2)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 2)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 2)
	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 4)
	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 4)
	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 6)

	// Set OAuth config to nil for second component
	rd = utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&v1.OAuth2{}, nil)
	sut = &oauthProxyResourceManager{rd, rr, s.kubeUtil, []ingress.AnnotationProvider{}, s.oauth2Config, "", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(component1Name, v1.OAuthProxyAuxiliaryComponentSuffix), actualDeploys.Items[0].Name)
	actualServices, _ = s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(utils.GetAuxOAuthProxyComponentServiceName(component1Name), actualServices.Items[0].Name)
	actualSecrets, _ = s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentSecretName(component1Name, v1.OAuthProxyAuxiliaryComponentSuffix), actualSecrets.Items[0].Name)
	actualRoles, _ = s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 2)
	actualRoleBindings, _ = s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 2)
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 4)
	var actualIngressNames []string
	for _, ing := range actualIngresses.Items {
		actualIngressNames = append(actualIngressNames, ing.Name)
	}
	s.ElementsMatch([]string{"ing1", oauthutil.GetAuxAuthProxyIngressName("ing1"), "ing2", "ing3"}, actualIngressNames)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetOwnerReferenceOfIngress() {
	actualOwnerReferences := ingress.GetOwnerReferenceOfIngress(&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "anyingress", UID: "anyuid"}})
	s.ElementsMatch([]metav1.OwnerReference{{APIVersion: networkingv1.SchemeGroupVersion.Identifier(), Kind: k8s.KindIngress, Name: "anyingress", UID: "anyuid", Controller: pointers.Ptr(true)}}, actualOwnerReferences)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetIngressName() {
	actualIngressName := oauthutil.GetAuxAuthProxyIngressName("ing")
	s.Equal(fmt.Sprintf("%s-%s", "ing", v1.OAuthProxyAuxiliaryComponentSuffix), actualIngressName)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect() {

	rd := utils.NewDeploymentBuilder().
		WithAppName("myapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("c1")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("c2")).
		BuildRD()

	s.addDeployment("d1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addDeployment("d6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	s.addSecret("sec1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addSecret("sec6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	s.addService("svc1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addService("svc2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addService("svc3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addService("svc4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addService("svc5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addService("svc6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addService("svc7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	s.addIngress("ing1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addIngress("ing6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	s.addRole("r1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addRole("r2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addRole("r3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addRole("r4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addRole("r5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRole("r6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addRole("r7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	s.addRoleBinding("rb1", "myapp-dev", "myapp", "c1", v1.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb2", "myapp-dev", "myapp", "c2", v1.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb3", "myapp-dev", "myapp", "c3", v1.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb4", "myapp-dev", "myapp", "c4", v1.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRoleBinding("rb6", "myapp-dev", "myapp2", "c6", v1.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb7", "myapp-qa", "myapp", "c7", v1.OAuthProxyAuxiliaryComponentType)

	sut := oauthProxyResourceManager{rd: rd, kubeutil: s.kubeUtil}
	err := sut.GarbageCollect(context.Background())
	s.Nil(err)

	actualDeployments, _ := s.kubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeployments.Items, 5)
	s.ElementsMatch([]string{"d1", "d2", "d5", "d6", "d7"}, s.getObjectNames(actualDeployments.Items))

	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 5)
	s.ElementsMatch([]string{"sec1", "sec2", "sec5", "sec6", "sec7"}, s.getObjectNames(actualSecrets.Items))

	actualServices, _ := s.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 5)
	s.ElementsMatch([]string{"svc1", "svc2", "svc5", "svc6", "svc7"}, s.getObjectNames(actualServices.Items))

	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 5)
	s.ElementsMatch([]string{"ing1", "ing2", "ing5", "ing6", "ing7"}, s.getObjectNames(actualIngresses.Items))

	actualRoles, _ := s.kubeClient.RbacV1().Roles(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 5)
	s.ElementsMatch([]string{"r1", "r2", "r5", "r6", "r7"}, s.getObjectNames(actualRoles.Items))

	actualRoleBindingss, _ := s.kubeClient.RbacV1().RoleBindings(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindingss.Items, 5)
	s.ElementsMatch([]string{"rb1", "rb2", "rb5", "rb6", "rb7"}, s.getObjectNames(actualRoleBindingss.Items))

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

func (s *OAuthProxyResourceManagerTestSuite) getObjectNames(items interface{}) []string {
	tItems := reflect.TypeOf(items)
	if tItems.Kind() != reflect.Slice {
		return nil
	}

	var names []string
	vItems := reflect.ValueOf(items)
	for i := 0; i < vItems.Len(); i++ {
		v := vItems.Index(i).Addr().Interface()
		if o, ok := v.(metav1.Object); ok {
			names = append(names, o.GetName())
		}

	}

	return names
}

func (*OAuthProxyResourceManagerTestSuite) getAppNameSelector(appName string) string {
	r, _ := labels.NewRequirement(kube.RadixAppLabel, selection.Equals, []string{appName})
	return r.String()
}

func (s *OAuthProxyResourceManagerTestSuite) addDeployment(name, namespace, appName, auxComponentName, auxComponentType string) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) addSecret(name, namespace, appName, auxComponentName, auxComponentType string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) addService(name, namespace, appName, auxComponentName, auxComponentType string) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) addIngress(name, namespace, appName, auxComponentName, auxComponentType string) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Create(context.Background(), ing, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) addRole(name, namespace, appName, auxComponentName, auxComponentType string) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.RbacV1().Roles(namespace).Create(context.Background(), role, metav1.CreateOptions{})

	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) addRoleBinding(name, namespace, appName, auxComponentName, auxComponentType string) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.RbacV1().RoleBindings(namespace).Create(context.Background(), roleBinding, metav1.CreateOptions{})

	s.Require().NoError(err)
}

func (s *OAuthProxyResourceManagerTestSuite) buildResourceLabels(appName, auxComponentName, auxComponentType string) labels.Set {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     auxComponentName,
		kube.RadixAuxiliaryComponentTypeLabel: auxComponentType,
	}
}
