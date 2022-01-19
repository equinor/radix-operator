package deployment

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
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
	secretProviderClient      secretProviderClient.Interface
	kubeUtil                  *kube.Kube
	ctrl                      *gomock.Controller
	ingressAnnotationProvider *MockIngressAnnotationProvider
	oauth2Config              *MockOAuth2Config
}

func TestOAuthProxyResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthProxyResourceManagerTestSuite))
}

func (*OAuthProxyResourceManagerTestSuite) SetupSuite() {
	os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, "1234-5678-91011")
	os.Setenv(defaults.OperatorDNSZoneEnvironmentVariable, dnsZone)
	os.Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, "app.dev.radix.equinor.com")
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "1")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	os.Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	os.Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler-server:main-latest")
	os.Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
	os.Setenv(defaults.RadixOAuthProxyImageEnvironmentVariable, "oauth:latest")
	os.Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, "oidc_issuer_url")
}

func (*OAuthProxyResourceManagerTestSuite) TearDownSuite() {
	os.Unsetenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
	os.Unsetenv(defaults.OperatorDNSZoneEnvironmentVariable)
	os.Unsetenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable)
	os.Unsetenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable)
	os.Unsetenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	os.Unsetenv(defaults.OperatorClusterTypeEnvironmentVariable)
	os.Unsetenv(defaults.RadixOAuthProxyImageEnvironmentVariable)
	os.Unsetenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)
}

func (s *OAuthProxyResourceManagerTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.secretProviderClient)
	s.ctrl = gomock.NewController(s.T())
	s.ingressAnnotationProvider = NewMockIngressAnnotationProvider(s.ctrl)
	s.oauth2Config = NewMockOAuth2Config(s.ctrl)
}

func (s *OAuthProxyResourceManagerTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *OAuthProxyResourceManagerTestSuite) TestNewOAuthProxyResourceManager() {
	rd := utils.NewDeploymentBuilder().BuildRD()
	rr := utils.NewRegistrationBuilder().BuildRR()
	oauthManager := NewOAuthProxyResourceManager(rd, rr, s.kubeUtil)
	sut, ok := oauthManager.(*oauthProxyResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
	s.NotNil(sut.oauth2Config)
	s.Len(sut.ingressAnnotationProviders, 1)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_NotPublicOrNoOAuth() {
	type scenarioDef struct{ rd *v1.RadixDeployment }
	scenarios := []scenarioDef{
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithPublicPort("http")).BuildRD()},
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234"}})).BuildRD()},
	}
	appName := "anyapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for _, scenario := range scenarios {
		sut := &oauthProxyResourceManager{scenario.rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
		err := sut.Sync()
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

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyDeploymentCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	inputOAuth := &v1.OAuth2{ClientID: "1234"}
	returnOAuth := &v1.OAuth2{
		ClientID:               test.RandomString(20),
		Scope:                  test.RandomString(20),
		SetXAuthRequestHeaders: utils.BoolPtr(true),
		SetAuthorizationHeader: utils.BoolPtr(false),
		ProxyPrefix:            test.RandomString(20),
		LoginURL:               test.RandomString(20),
		RedeemURL:              test.RandomString(20),
		SessionStoreType:       "redis",
		Cookie: &v1.OAuth2Cookie{
			Name:     test.RandomString(20),
			Expire:   test.RandomString(20),
			Refresh:  test.RandomString(20),
			SameSite: v1.CookieSameSiteType(test.RandomString(20)),
		},
		CookieStore: &v1.OAuth2CookieStore{
			Minimal: utils.BoolPtr(true),
		},
		RedisStore: &v1.OAuth2RedisStore{
			ConnectionURL: test.RandomString(20),
		},
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               test.RandomString(20),
			JWKSURL:                 test.RandomString(20),
			SkipDiscovery:           utils.BoolPtr(true),
			InsecureSkipVerifyNonce: utils.BoolPtr(false),
		},
	}
	s.oauth2Config.EXPECT().MergeWithDefaults(inputOAuth).Times(1).Return(returnOAuth)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: inputOAuth})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)

	actualDeploy := actualDeploys.Items[0]
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix), actualDeploy.Name)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualDeploy.OwnerReferences)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType}
	s.Equal(expectedLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedLabels, actualDeploy.Spec.Template.Labels)

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable), defaultContainer.Image)

	s.Len(defaultContainer.Ports, 1)
	s.Equal(oauthProxyPortNumber, defaultContainer.Ports[0].ContainerPort)
	s.Equal(oauthProxyPortName, defaultContainer.Ports[0].Name)
	s.NotNil(defaultContainer.ReadinessProbe)
	s.Equal(oauthProxyPortNumber, defaultContainer.ReadinessProbe.TCPSocket.Port.IntVal)

	s.Len(defaultContainer.Env, 29)
	s.Equal("oidc", s.getEnvVarValueByName("OAUTH2_PROXY_PROVIDER", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_HTTPONLY", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_SECURE", defaultContainer.Env))
	s.Equal("false", s.getEnvVarValueByName("OAUTH2_PROXY_PASS_BASIC_AUTH", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_SKIP_PROVIDER_BUTTON", defaultContainer.Env))
	s.Equal("*", s.getEnvVarValueByName("OAUTH2_PROXY_EMAIL_DOMAINS", defaultContainer.Env))
	s.Equal(fmt.Sprintf("http://:%v", oauthProxyPortNumber), s.getEnvVarValueByName("OAUTH2_PROXY_HTTP_ADDRESS", defaultContainer.Env))
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
	secretName := utils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthCookieSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName("OAUTH2_PROXY_COOKIE_SECRET", defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthClientSecretKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName("OAUTH2_PROXY_CLIENT_SECRET", defaultContainer.Env))
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthRedisPasswordKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName("OAUTH2_PROXY_REDIS_PASSWORD", defaultContainer.Env))

	// Env var OAUTH2_PROXY_REDIS_PASSWORD should not be present when SessionStoreType is cookie
	returnOAuth.SessionStoreType = "cookie"
	s.oauth2Config.EXPECT().MergeWithDefaults(inputOAuth).Times(1).Return(returnOAuth)
	err = sut.Sync()
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env, 28)
	s.False(s.getEnvVarExist("OAUTH2_PROXY_REDIS_PASSWORD", actualDeploys.Items[0].Spec.Template.Spec.Containers[0].Env))
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxySecretAndRbacCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(1).Return(&v1.OAuth2{})

	rr := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups([]string{"ad1", "ad2"}).WithMachineUser(true).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType}
	expectedSecretName := utils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(expectedSecretName, actualSecrets.Items[0].Name)
	s.Equal(expectedLabels, actualSecrets.Items[0].Labels)
	s.NotEmpty(actualSecrets.Items[0].Data[defaults.OAuthCookieSecretKeyName])

	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 1)
	s.Equal(sut.getRoleAndRoleBindingName(componentName), actualRoles.Items[0].Name)
	s.Equal(expectedLabels, actualRoles.Items[0].Labels)
	s.Len(actualRoles.Items[0].Rules, 1)
	s.ElementsMatch([]string{expectedSecretName}, actualRoles.Items[0].Rules[0].ResourceNames)

	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 1)
	s.Equal(sut.getRoleAndRoleBindingName(componentName), actualRoleBindings.Items[0].Name)
	s.Equal(expectedLabels, actualRoleBindings.Items[0].Labels)
	s.Equal(actualRoles.Items[0].Name, actualRoleBindings.Items[0].RoleRef.Name)
	s.Len(actualRoleBindings.Items[0].Subjects, 3)
	expectedSubjects := []rbacv1.Subject{
		{Kind: "ServiceAccount", Name: fmt.Sprintf("%s-machine-user", appName), Namespace: utils.GetAppNamespace(appName)},
		{Kind: "Group", APIGroup: "rbac.authorization.k8s.io", Name: "ad1"},
		{Kind: "Group", APIGroup: "rbac.authorization.k8s.io", Name: "ad2"},
	}
	s.ElementsMatch(expectedSubjects, actualRoleBindings.Items[0].Subjects)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxySecret_KeysGarbageCollected() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(1).Return(&v1.OAuth2{SessionStoreType: v1.SessionStoreRedis})

	secretName := utils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
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
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err = sut.Sync()
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
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(1).Return(&v1.OAuth2{SessionStoreType: v1.SessionStoreCookie})
	sut.Sync()
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
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(1).Return(&v1.OAuth2{})

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType}
	expectedServiceName := utils.GetAuxiliaryComponentServiceName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(expectedServiceName, actualServices.Items[0].Name)
	s.Equal(expectedLabels, actualServices.Items[0].Labels)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualServices.Items[0].OwnerReferences)
	s.Equal(corev1.ServiceTypeClusterIP, actualServices.Items[0].Spec.Type)
	s.Len(actualServices.Items[0].Spec.Ports, 1)
	s.Equal(corev1.ServicePort{Port: oauthProxyPortNumber, TargetPort: intstr.FromString(oauthProxyPortName), Protocol: corev1.ProtocolTCP}, actualServices.Items[0].Spec.Ports[0])
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyIngressesCreated() {
	appName, envName, component1Name, component2Name := "anyapp", "qa", "server", "web"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	ingServer := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing1", Labels: map[string]string{kube.RadixComponentLabel: component1Name}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingServerNoRules := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing2", Labels: map[string]string{kube.RadixComponentLabel: component1Name}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
		},
	}
	ingOtherComponent := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing3", Labels: map[string]string{kube.RadixComponentLabel: "othercomponent"}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingServerOtherNs := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing4", Labels: map[string]string{kube.RadixComponentLabel: component1Name}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing1-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing1.local"}},
		},
	}
	ingWeb1 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing5", Labels: map[string]string{kube.RadixComponentLabel: component2Name}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass1"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing2-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing2.local"}},
		},
	}
	ingWeb2 := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing6", Labels: map[string]string{kube.RadixComponentLabel: component2Name}},
		Spec: networkingv1.IngressSpec{
			IngressClassName: utils.StringPtr("anyclass2"),
			TLS:              []networkingv1.IngressTLS{{SecretName: "ing2-tls"}},
			Rules:            []networkingv1.IngressRule{{Host: "ing2.public"}},
		},
	}
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingServer, metav1.CreateOptions{})
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingServerNoRules, metav1.CreateOptions{})
	s.kubeClient.NetworkingV1().Ingresses("otherns").Create(context.Background(), &ingServerOtherNs, metav1.CreateOptions{})
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingOtherComponent, metav1.CreateOptions{})
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb1, metav1.CreateOptions{})
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(context.Background(), &ingWeb2, metav1.CreateOptions{})

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWithDefaults(rd.Spec.Components[0].Authentication.OAuth2).Times(1).Return(&v1.OAuth2{ProxyPrefix: "auth1"})
	s.oauth2Config.EXPECT().MergeWithDefaults(rd.Spec.Components[1].Authentication.OAuth2).Times(1).Return(&v1.OAuth2{ProxyPrefix: "auth2"})

	expectedIngServerCall := rd.Spec.Components[0].DeepCopy()
	expectedIngServerCall.Authentication.OAuth2.ProxyPrefix = "auth1"
	expectedIngServerAnnotations := map[string]string{"annotation1-1": "val1-1", "annotation1-2": "val1-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngServerCall).Times(1).Return(expectedIngServerAnnotations)
	expectedIngWebCall := rd.Spec.Components[1].DeepCopy()
	expectedIngWebCall.Authentication.OAuth2.ProxyPrefix = "auth2"
	expectedIngWebAnnotations := map[string]string{"annotation2-1": "val2-1", "annotation2-2": "val2-2"}
	s.ingressAnnotationProvider.EXPECT().GetAnnotations(expectedIngWebCall).Times(2).Return(expectedIngWebAnnotations)
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{s.ingressAnnotationProvider}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 9)
	getIngress := func(name string, ingresses []networkingv1.Ingress) *networkingv1.Ingress {
		for _, ingress := range ingresses {
			if ingress.Name == name {
				return &ingress
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
							Name: utils.GetAuxiliaryComponentServiceName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix),
							Port: networkingv1.ServiceBackendPort{Number: oauthProxyPortNumber},
						},
					},
				}},
			},
		}
	}
	// Ingresses for server component
	actualIngress := getIngress(fmt.Sprintf("%s-%s", ingServer.Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualIngresses.Items)
	s.NotNil(actualIngress)
	s.Equal(expectedIngServerAnnotations, actualIngress.Annotations)
	s.ElementsMatch(sut.getOwnerReferenceOfIngress(&ingServer), actualIngress.OwnerReferences)
	s.Equal(ingServer.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingServer.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingServer.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component1Name, "auth1"), actualIngress.Spec.Rules[0].IngressRuleValue)

	// Ingresses for web component
	actualIngress = getIngress(fmt.Sprintf("%s-%s", ingWeb1.Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualIngresses.Items)
	s.NotNil(actualIngress)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(sut.getOwnerReferenceOfIngress(&ingWeb1), actualIngress.OwnerReferences)
	s.Equal(ingWeb1.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb1.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb1.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)

	actualIngress = getIngress(fmt.Sprintf("%s-%s", ingWeb2.Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualIngresses.Items)
	s.NotNil(actualIngress)
	s.Equal(expectedIngWebAnnotations, actualIngress.Annotations)
	s.ElementsMatch(sut.getOwnerReferenceOfIngress(&ingWeb2), actualIngress.OwnerReferences)
	s.Equal(ingWeb2.Spec.IngressClassName, actualIngress.Spec.IngressClassName)
	s.Equal(ingWeb2.Spec.TLS, actualIngress.Spec.TLS)
	s.Equal(ingWeb2.Spec.Rules[0].Host, actualIngress.Spec.Rules[0].Host)
	s.Equal(getExpectedIngressRule(component2Name, "auth2"), actualIngress.Spec.Rules[0].IngressRuleValue)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_Sync_OAuthProxyUninstall() {
	appName, envName, component1Name, component2Name := "anyapp", "qa", "server", "web"
	envNs := utils.GetEnvironmentNamespace(appName, envName)

	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing1", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component1Name}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing2", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)
	s.kubeClient.NetworkingV1().Ingresses(envNs).Create(
		context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing3", Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: component2Name}},
			Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "anyhost"}}}},
		metav1.CreateOptions{},
	)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(2).Return(&v1.OAuth2{})
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	// Bootstrap oauth proxy resources for two components
	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 2)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 2)
	actualSecrets, _ := s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 2)
	actualRoles, _ := s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 2)
	actualRoleBindings, _ := s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 2)
	actualIngresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 6)

	// Set OAuth config to nil for second component
	rd = utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{})).
		BuildRD()
	s.oauth2Config.EXPECT().MergeWithDefaults(gomock.Any()).Times(1).Return(&v1.OAuth2{})
	sut = &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotationProvider{}, s.oauth2Config}
	err = sut.Sync()
	s.Nil(err)
	actualDeploys, _ = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(component1Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualDeploys.Items[0].Name)
	actualServices, _ = s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentServiceName(component1Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualServices.Items[0].Name)
	actualSecrets, _ = s.kubeClient.CoreV1().Secrets(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualSecrets.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentSecretName(component1Name, defaults.OAuthProxyAuxiliaryComponentSuffix), actualSecrets.Items[0].Name)
	actualRoles, _ = s.kubeClient.RbacV1().Roles(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoles.Items, 1)
	s.Equal(sut.getRoleAndRoleBindingName(component1Name), actualRoles.Items[0].Name)
	actualRoleBindings, _ = s.kubeClient.RbacV1().RoleBindings(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualRoleBindings.Items, 1)
	s.Equal(sut.getRoleAndRoleBindingName(component1Name), actualRoleBindings.Items[0].Name)
	actualIngresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualIngresses.Items, 4)
	var actualIngressNames []string
	for _, ing := range actualIngresses.Items {
		actualIngressNames = append(actualIngressNames, ing.Name)
	}
	s.ElementsMatch([]string{"ing1", sut.getIngressName("ing1"), "ing2", "ing3"}, actualIngressNames)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetOwnerReferenceOfIngress() {
	sut := &oauthProxyResourceManager{}
	actualOwnerReferences := sut.getOwnerReferenceOfIngress(&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "anyingress", UID: "anyuid"}})
	s.ElementsMatch([]metav1.OwnerReference{{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "anyingress", UID: "anyuid", Controller: utils.BoolPtr(true)}}, actualOwnerReferences)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetRoleAndRoleBindingName() {
	sut := &oauthProxyResourceManager{}
	actualRoleName := sut.getRoleAndRoleBindingName("component")
	s.Equal(fmt.Sprintf("radix-app-adm-component-%s", defaults.OAuthProxyAuxiliaryComponentSuffix), actualRoleName)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GetIngressName() {
	sut := &oauthProxyResourceManager{}
	actualIngressName := sut.getIngressName("ing")
	s.Equal(fmt.Sprintf("%s-%s", "ing", defaults.OAuthProxyAuxiliaryComponentSuffix), actualIngressName)
}

func (s *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect() {

	rd := utils.NewDeploymentBuilder().
		WithAppName("myapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("c1")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("c2")).
		BuildRD()

	s.addDeployment("d1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addDeployment("d6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addDeployment("d7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	s.addSecret("sec1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addSecret("sec6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addSecret("sec7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	s.addService("svc1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addService("svc2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addService("svc3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addService("svc4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addService("svc5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addService("svc6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addService("svc7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	s.addIngress("ing1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addIngress("ing6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addIngress("ing7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	s.addRole("r1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRole("r2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRole("r3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRole("r4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRole("r5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRole("r6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRole("r7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	s.addRoleBinding("rb1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addRoleBinding("rb6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	s.addRoleBinding("rb7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	sut := oauthProxyResourceManager{rd: rd, kubeutil: s.kubeUtil}
	err := sut.GarbageCollect()
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
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
}

func (s *OAuthProxyResourceManagerTestSuite) addIngress(name, namespace, appName, auxComponentName, auxComponentType string) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
}

func (s *OAuthProxyResourceManagerTestSuite) addRoleBinding(name, namespace, appName, auxComponentName, auxComponentType string) {
	rolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.RbacV1().RoleBindings(namespace).Create(context.Background(), rolebinding, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (s *OAuthProxyResourceManagerTestSuite) buildResourceLabels(appName, auxComponentName, auxComponentType string) labels.Set {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     auxComponentName,
		kube.RadixAuxiliaryComponentTypeLabel: auxComponentType,
	}
}
