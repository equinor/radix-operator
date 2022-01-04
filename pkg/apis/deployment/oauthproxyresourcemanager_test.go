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
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type OAuthProxyResourceManagerTestSuite struct {
	suite.Suite
	kubeClient         kubernetes.Interface
	radixClient        radixclient.Interface
	kubeUtil           *kube.Kube
	ctrl               *gomock.Controller
	ingressAnnotations *MockIngressAnnotations
	oauth2Config       *MockOAuth2Config
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
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient)
	s.ctrl = gomock.NewController(s.T())
	s.ingressAnnotations = NewMockIngressAnnotations(s.ctrl)
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
	s.Len(sut.ingressAnnotations, 1)
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
		sut := &oauthProxyResourceManager{scenario.rd, rr, s.kubeUtil, []IngressAnnotations{s.ingressAnnotations}, s.oauth2Config}
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
			SameSite: test.RandomString(20),
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
	sut := &oauthProxyResourceManager{rd, rr, s.kubeUtil, []IngressAnnotations{s.ingressAnnotations}, s.oauth2Config}
	err := sut.Sync()
	s.Nil(err)

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Len(actualDeploys.Items, 1)

	actualDeploy := actualDeploys.Items[0]
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix), actualDeploy.Name)
	s.Equal(utils.GetEnvironmentNamespace(appName, envName), actualDeploy.Namespace)
	s.ElementsMatch(getOwnerReferenceOfDeployment(rd), actualDeploy.OwnerReferences)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType}
	s.Equal(expectedLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedLabels, actualDeploy.Spec.Template.Labels)

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable), defaultContainer.Image)

	s.Len(defaultContainer.Ports, 1)
	s.Equal(oauthProxyPortNumber, defaultContainer.Ports[0].ContainerPort)
	s.Equal(oauthProxyPortName, defaultContainer.Ports[0].Name)
	readyProbe, err := getReadinessProbe(oauthProxyPortNumber)
	s.Nil(err)
	s.Equal(readyProbe, defaultContainer.ReadinessProbe)

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
	s.Equal(returnOAuth.Cookie.SameSite, s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_SAMESITE", defaultContainer.Env))
	s.Equal("true", s.getEnvVarValueByName("OAUTH2_PROXY_COOKIE_MINIMAL", defaultContainer.Env))
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
