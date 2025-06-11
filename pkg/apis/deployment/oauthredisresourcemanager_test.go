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

type OAuthRedisResourceManagerTestSuite struct {
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

func TestOAuthRedisResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthRedisResourceManagerTestSuite))
}

func (s *OAuthRedisResourceManagerTestSuite) SetupSuite() {
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

func (s *OAuthRedisResourceManagerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *OAuthRedisResourceManagerTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	s.ctrl = gomock.NewController(s.T())
}

func (s *OAuthRedisResourceManagerTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *OAuthRedisResourceManagerTestSuite) TestNewOAuthRedisResourceManager() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	rd := utils.NewDeploymentBuilder().BuildRD()
	rr := utils.NewRegistrationBuilder().BuildRR()

	oauthManager := NewOAuthRedisResourceManager(rd, rr, s.kubeUtil, "redis:123")
	sut, ok := oauthManager.(*oauthRedisResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
	s.Equal("redis:123", sut.oauth2RedisDockerImage)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_ComponentRestartEnvVar() {
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
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			envVarExist := slice.Any(deploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool { return ev.Name == defaults.RadixRestartEnvironmentVariable })
			s.Equal(test.expectRestartEnvVar, envVarExist)
		})
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_NotPublicOrNoOAuth() {
	appName := "anyapp"
	type scenarioDef struct{ rd *v1.RadixDeployment }
	scenarios := []scenarioDef{
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithPublicPort("http")).BuildRD()},
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234"}})).BuildRD()},
	}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for _, scenario := range scenarios {
		sut := &oauthRedisResourceManager{scenario.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
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

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OauthDeploymentReplicas() {
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
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := sut.kubeutil.KubeClient().AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			s.Equal(test.expectedReplicas, *deploys.Items[0].Spec.Replicas)
		})
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisDeploymentCreated() {
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
		SessionStoreType:       v1.SessionStoreSystemManaged,
		Cookie: &v1.OAuth2Cookie{
			Name:     commonUtils.RandString(20),
			Expire:   commonUtils.RandString(20),
			Refresh:  commonUtils.RandString(20),
			SameSite: v1.CookieSameSiteType(commonUtils.RandString(20)),
		},
		CookieStore: &v1.OAuth2CookieStore{
			Minimal: pointers.Ptr(true),
		},
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               commonUtils.RandString(20),
			JWKSURL:                 commonUtils.RandString(20),
			SkipDiscovery:           pointers.Ptr(true),
			InsecureSkipVerifyNonce: pointers.Ptr(false),
		},
		SkipAuthRoutes: []string{"POST=^/api/public-entity/?$", "GET=^/skip/auth/routes/get", "!=^/api"},
	}

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: inputOAuth}).WithRuntime(&v1.Runtime{Architecture: "customarch"})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)

	actualDeploy := actualDeploys.Items[0]
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix), actualDeploy.Name)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualDeploy.OwnerReferences)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	s.Equal(expectedLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedLabels, actualDeploy.Spec.Template.Labels)

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(sut.oauth2RedisDockerImage, defaultContainer.Image)

	s.Len(defaultContainer.Ports, 1)
	s.Equal(v1.OAuthRedisPortNumber, defaultContainer.Ports[0].ContainerPort)
	s.Equal("redis", defaultContainer.Ports[0].Name)
	s.NotNil(defaultContainer.ReadinessProbe)
	s.Equal(v1.OAuthRedisPortNumber, defaultContainer.ReadinessProbe.TCPSocket.Port.IntVal)

	expectedAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
			{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
			{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
		}}}}},
	}
	s.Equal(expectedAffinity, actualDeploy.Spec.Template.Spec.Affinity, "oauth2 aux deployment must not use component's runtime config")

	s.Len(defaultContainer.Env, 2)
	secretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthRedisPasswordKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(oauth2ProxyRedisPasswordEnvironmentVariable, defaultContainer.Env))

	// Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie
	returnOAuth.SessionStoreType = "cookie"
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 0)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisSecretAndRbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}
	// readerGroups, readerUsers := []string{"rdr1", "rdr2"}, []string{"rdrUsr1", "rdrUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(expectedSecretName, actualSecrets.Items[0].Name)
	s.Equal(expectedLabels, actualSecrets.Items[0].Labels)
	s.NotEmpty(actualSecrets.Items[0].Data[defaults.OAuthCookieSecretKeyName])
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisRbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	adminGroups, adminUsers := []string{"adm1", "adm2"}, []string{"admUsr1", "admUsr2"}
	readerGroups, readerUsers := []string{"rdr1", "rdr2"}, []string{"rdrUsr1", "rdrUsr2"}

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithAdUsers(adminUsers).WithReaderAdGroups(readerGroups).WithReaderAdUsers(readerUsers).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedRoles := []string{fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)), fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix))}
	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)

	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleNames(actualRoles))

	admRole := getRoleByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, admRole.Labels)
	s.Len(admRole.Rules, 1)
	s.ElementsMatch([]string{""}, admRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, admRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, admRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "update", "patch", "list", "watch", "delete"}, admRole.Rules[0].Verbs)

	readerRole := getRoleByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)), actualRoles)
	s.Equal(expectedLabels, readerRole.Labels)
	s.Len(readerRole.Rules, 1)
	s.ElementsMatch([]string{""}, readerRole.Rules[0].APIGroups)
	s.ElementsMatch([]string{"secrets"}, readerRole.Rules[0].Resources)
	s.ElementsMatch([]string{expectedSecretName}, readerRole.Rules[0].ResourceNames)
	s.ElementsMatch([]string{"get", "list", "watch"}, readerRole.Rules[0].Verbs)

	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.ElementsMatch(expectedRoles, getRoleBindingNames(actualRoleBindings))

	admRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-adm-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)), actualRoleBindings)
	s.Equal(expectedLabels, admRoleBinding.Labels)
	s.Equal(admRole.Name, admRoleBinding.RoleRef.Name)
	expectedSubjects := []rbacv1.Subject{
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm1"},
		{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "adm2"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr1"},
		{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: "admUsr2"},
	}
	s.ElementsMatch(expectedSubjects, admRoleBinding.Subjects)

	readerRoleBinding := getRoleBindingByName(fmt.Sprintf("radix-app-reader-%s", utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)), actualRoleBindings)
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

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisServiceCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	expectedServiceName := utils.GetAuxOAuthRedisServiceName(componentName)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(expectedServiceName, actualServices.Items[0].Name)
	s.Equal(expectedLabels, actualServices.Items[0].Labels)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualServices.Items[0].OwnerReferences)
	s.Equal(corev1.ServiceTypeClusterIP, actualServices.Items[0].Spec.Type)
	s.Len(actualServices.Items[0].Spec.Ports, 1)
	s.Equal(corev1.ServicePort{Port: v1.OAuthRedisPortNumber, TargetPort: intstr.FromString("redis"), Protocol: corev1.ProtocolTCP}, actualServices.Items[0].Spec.Ports[0])
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisIngressesCreated() {
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

	expectedIngServerCall := rd.Spec.Components[0].DeepCopy()
	expectedIngServerCall.Authentication.OAuth2.ProxyPrefix = "auth1"
	expectedIngServerAnnotations := map[string]string{"annotation1-1": "val1-1", "annotation1-2": "val1-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngServerCall, rd.Namespace).Times(1).Return(expectedIngServerAnnotations, nil)
	expectedIngWebCall := rd.Spec.Components[1].DeepCopy()
	expectedIngWebCall.Authentication.OAuth2.ProxyPrefix = "auth2"
	expectedIngWebAnnotations := map[string]string{"annotation2-1": "val2-1", "annotation2-2": "val2-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngWebCall, rd.Namespace).Times(5).Return(expectedIngWebAnnotations, nil)
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
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
	getExpectedIngressRule := func(componentName, redisPrefix string) networkingv1.IngressRuleValue {
		pathType := networkingv1.PathTypeImplementationSpecific
		return networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     oauthutil.SanitizePathPrefix(redisPrefix),
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: utils.GetAuxOAuthRedisServiceName(componentName),
							Port: networkingv1.ServiceBackendPort{Number: v1.OAuthRedisPortNumber},
						},
					},
				}},
			},
		}
	}
	// Ingresses for server component
	ingressName := fmt.Sprintf("%s-%s", ingServer.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress := getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngServerAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingServer), actualIngress.OwnerReferences)
	s.Equal(ingServer.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingServer.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingServer.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component1Name, "auth1"), actualIngress.Spec.Rules[0].IngressRuleValue)

	// Ingresses for web component
	ingressName = fmt.Sprintf("%s-%s", ingWeb1.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb1), actualIngress.OwnerReferences)
	s.Equal(ingWeb1.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb1.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb1.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb2.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb2), actualIngress.OwnerReferences)
	s.Equal(ingWeb2.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb2.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb2.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb3.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb3), actualIngress.OwnerReferences)
	s.Equal(ingWeb3.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb3.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb3.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb4.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb4), actualIngress.OwnerReferences)
	s.Equal(ingWeb4.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb4.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb4.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	ingressName = fmt.Sprintf("%s-%s", ingWeb5.Name, v1.OAuthRedisAuxiliaryComponentSuffix)
	actualIngress = getIngress(ingressName, actualIngresses.Items)
	s.Require().NotNil(actualIngress, "not found aux ingress %s", ingressName)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(ingress.GetOwnerReferenceOfIngress(&ingWeb5), actualIngress.OwnerReferences)
	s.Equal(ingWeb5.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb5.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb5.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisUninstall() {
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
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.NoError(err)

	// Bootstrap oauth redis resources for two components
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
	sut = &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(component1Name, v1.OAuthRedisAuxiliaryComponentSuffix), actualDeploys.Items[0].Name)
	actualServices, _ = s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(utils.GetAuxOAuthRedisServiceName(component1Name), actualServices.Items[0].Name)
	actualRoles, _ = s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 2)
	actualRoleBindings, _ = s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 2)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_GetOwnerReferenceOfIngress() {
	actualOwnerReferences := ingress.GetOwnerReferenceOfIngress(&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "anyingress", UID: "anyuid"}})
	s.ElementsMatch([]metav1.OwnerReference{{APIVersion: networkingv1.SchemeGroupVersion.Identifier(), Kind: k8s.KindIngress, Name: "anyingress", UID: "anyuid", Controller: pointers.Ptr(true)}}, actualOwnerReferences)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_GarbageCollect() {

	rd := utils.NewDeploymentBuilder().
		WithAppName("myapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("c1")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("c2")).
		BuildRD()

	s.addDeployment("d1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addDeployment("d6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addSecret("sec1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addSecret("sec2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addSecret("sec3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addSecret("sec4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addSecret("sec5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addSecret("sec6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addSecret("sec7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addService("svc1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addService("svc6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addIngress("ing1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addIngress("ing2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addIngress("ing3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addIngress("ing4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addIngress("ing5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addIngress("ing6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addIngress("ing7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addRole("r1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addRole("r2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addRole("r3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addRole("r4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addRole("r5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRole("r6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addRole("r7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addRoleBinding("rb1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addRoleBinding("rb2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addRoleBinding("rb3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addRoleBinding("rb4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addRoleBinding("rb5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRoleBinding("rb6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addRoleBinding("rb7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	sut := oauthRedisResourceManager{rd: rd, kubeutil: s.kubeUtil}
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

func (*OAuthRedisResourceManagerTestSuite) getEnvVarExist(name string, envvars []corev1.EnvVar) bool {
	for _, envvar := range envvars {
		if envvar.Name == name {
			return true
		}
	}
	return false
}

func (*OAuthRedisResourceManagerTestSuite) getEnvVarValueByName(name string, envvars []corev1.EnvVar) string {
	for _, envvar := range envvars {
		if envvar.Name == name {
			return envvar.Value
		}
	}
	return ""
}

func (*OAuthRedisResourceManagerTestSuite) getEnvVarValueFromByName(name string, envvars []corev1.EnvVar) corev1.EnvVarSource {
	for _, envvar := range envvars {
		if envvar.Name == name && envvar.ValueFrom != nil {
			return *envvar.ValueFrom
		}
	}
	return corev1.EnvVarSource{}
}

func (s *OAuthRedisResourceManagerTestSuite) getObjectNames(items interface{}) []string {
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

func (*OAuthRedisResourceManagerTestSuite) getAppNameSelector(appName string) string {
	r, _ := labels.NewRequirement(kube.RadixAppLabel, selection.Equals, []string{appName})
	return r.String()
}

func (s *OAuthRedisResourceManagerTestSuite) addDeployment(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) addSecret(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) addService(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) addIngress(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) addRole(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) addRoleBinding(name, namespace, appName, auxComponentName, auxComponentType string) {
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

func (s *OAuthRedisResourceManagerTestSuite) buildResourceLabels(appName, auxComponentName, auxComponentType string) labels.Set {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     auxComponentName,
		kube.RadixAuxiliaryComponentTypeLabel: auxComponentType,
	}
}
