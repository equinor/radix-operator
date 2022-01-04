package deployment

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type OAuthProxyResourceManagerTestSuite struct {
	suite.Suite
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
	kubeUtil    *kube.Kube
}

func TestOAuthProxyResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthProxyResourceManagerTestSuite))
}

func (suite *OAuthProxyResourceManagerTestSuite) SetupSuite() {
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

func (suite *OAuthProxyResourceManagerTestSuite) TearDownSuite() {
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

func (suite *OAuthProxyResourceManagerTestSuite) SetupTest() {
	suite.kubeClient = kubefake.NewSimpleClientset()
	suite.radixClient = radixfake.NewSimpleClientset()
	suite.kubeUtil, _ = kube.New(suite.kubeClient, suite.radixClient)
}

func (suite *OAuthProxyResourceManagerTestSuite) Test_Sync_ResourcesCreated() {
	rr := utils.NewRegistrationBuilder().WithName("anyapp").BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName("anyapp").
		WithEnvironment("qa").
		WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithPublicPort("http")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("notpublic").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "anyclientid"}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName("default").WithPublicPort("http").WithAuthentication(&v1.Authentication{
			OAuth2: &v1.OAuth2{
				ClientID: "1234",
			}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName("overrides").WithPublicPort("http").WithAuthentication(&v1.Authentication{
			OAuth2: &v1.OAuth2{
				ClientID: "1234",
			}})).
		BuildRD()
	sut := NewOAuthProxyResourceManager(rd, rr, suite.kubeUtil)
	err := sut.Sync()

	suite.Nil(err)
	actualDeploys, _ := suite.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualDeploys.Items, 2)

	defaultDeploy := getDeploymentByName(utils.GetAuxiliaryComponentDeploymentName("default", defaults.OAuthProxyAuxiliaryComponentSuffix), actualDeploys)
	suite.NotNil(defaultDeploy)
	expectedDefaultDeployLabels := map[string]string{kube.RadixAppLabel: "anyapp", kube.RadixAuxiliaryComponentLabel: "default", kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType}
	suite.Equal(expectedDefaultDeployLabels, defaultDeploy.Labels)
	suite.Len(defaultDeploy.Spec.Template.Spec.Containers, 1)
	suite.Equal(expectedDefaultDeployLabels, defaultDeploy.Spec.Template.Labels)
	defaultContainer := defaultDeploy.Spec.Template.Spec.Containers[0]
	suite.Len(defaultContainer.Ports, 1)
	suite.Equal(oauthProxyPortNumber, defaultContainer.Ports[0].ContainerPort)
	suite.Equal(oauthProxyPortName, defaultContainer.Ports[0].Name)
	suite.Len(defaultContainer.Env, 21)
}

func (suite *OAuthProxyResourceManagerTestSuite) Test_Sync_ResourcesGarbageCollectedForExistingComponent() {

}

func (suite *OAuthProxyResourceManagerTestSuite) Test_Sync_SecretKeysDeleted() {

}

func (suite *OAuthProxyResourceManagerTestSuite) Test_GarbageCollect() {

	rd := utils.NewDeploymentBuilder().
		WithAppName("myapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("c1")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("c2")).
		BuildRD()

	suite.addDeployment("d1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addDeployment("d2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addDeployment("d3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addDeployment("d4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addDeployment("d5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addDeployment("d6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addDeployment("d7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	suite.addSecret("sec1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addSecret("sec2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addSecret("sec3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addSecret("sec4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addSecret("sec5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addSecret("sec6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addSecret("sec7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	suite.addService("svc1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addService("svc2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addService("svc3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addService("svc4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addService("svc5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addService("svc6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addService("svc7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	suite.addIngress("ing1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addIngress("ing2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addIngress("ing3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addIngress("ing4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addIngress("ing5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addIngress("ing6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addIngress("ing7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	suite.addRole("r1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRole("r2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRole("r3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRole("r4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRole("r5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addRole("r6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRole("r7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	suite.addRoleBinding("rb1", "myapp-dev", "myapp", "c1", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRoleBinding("rb2", "myapp-dev", "myapp", "c2", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRoleBinding("rb3", "myapp-dev", "myapp", "c3", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRoleBinding("rb4", "myapp-dev", "myapp", "c4", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRoleBinding("rb5", "myapp-dev", "myapp", "c5", "anyauxtype")
	suite.addRoleBinding("rb6", "myapp-dev", "myapp2", "c6", defaults.OAuthProxyAuxiliaryComponentType)
	suite.addRoleBinding("rb7", "myapp-qa", "myapp", "c7", defaults.OAuthProxyAuxiliaryComponentType)

	sut := oauthProxyResourceManager{rd: rd, kubeutil: suite.kubeUtil}
	err := sut.GarbageCollect()
	suite.Nil(err)

	actualDeployments, _ := suite.kubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualDeployments.Items, 5)
	suite.ElementsMatch([]string{"d1", "d2", "d5", "d6", "d7"}, suite.getObjectNames(actualDeployments.Items))

	actualSecrets, _ := suite.kubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualSecrets.Items, 5)
	suite.ElementsMatch([]string{"sec1", "sec2", "sec5", "sec6", "sec7"}, suite.getObjectNames(actualSecrets.Items))

	actualServices, _ := suite.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualServices.Items, 5)
	suite.ElementsMatch([]string{"svc1", "svc2", "svc5", "svc6", "svc7"}, suite.getObjectNames(actualServices.Items))

	actualIngresses, _ := suite.kubeClient.NetworkingV1().Ingresses(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualIngresses.Items, 5)
	suite.ElementsMatch([]string{"ing1", "ing2", "ing5", "ing6", "ing7"}, suite.getObjectNames(actualIngresses.Items))

	actualRoles, _ := suite.kubeClient.RbacV1().Roles(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualRoles.Items, 5)
	suite.ElementsMatch([]string{"r1", "r2", "r5", "r6", "r7"}, suite.getObjectNames(actualRoles.Items))

	actualRoleBindingss, _ := suite.kubeClient.RbacV1().RoleBindings(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	suite.Len(actualRoleBindingss.Items, 5)
	suite.ElementsMatch([]string{"rb1", "rb2", "rb5", "rb6", "rb7"}, suite.getObjectNames(actualRoleBindingss.Items))

}

func (suite *OAuthProxyResourceManagerTestSuite) getObjectNames(items interface{}) []string {
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

func (suite *OAuthProxyResourceManagerTestSuite) addDeployment(name, namespace, appName, auxComponentName, auxComponentType string) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) addSecret(name, namespace, appName, auxComponentName, auxComponentType string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) addService(name, namespace, appName, auxComponentName, auxComponentType string) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) addIngress(name, namespace, appName, auxComponentName, auxComponentType string) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) addRole(name, namespace, appName, auxComponentName, auxComponentType string) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.RbacV1().Roles(namespace).Create(context.Background(), role, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) addRoleBinding(name, namespace, appName, auxComponentName, auxComponentType string) {
	rolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    suite.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := suite.kubeClient.RbacV1().RoleBindings(namespace).Create(context.Background(), rolebinding, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (suite *OAuthProxyResourceManagerTestSuite) buildResourceLabels(appName, auxComponentName, auxComponentType string) labels.Set {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     auxComponentName,
		kube.RadixAuxiliaryComponentTypeLabel: auxComponentType,
	}
}
