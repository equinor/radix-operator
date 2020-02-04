package deployment

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	kubeUtils "github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils/maps"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	extension "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const anyContainerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface, *kubeUtils.Kube, radixclient.Interface) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kubeUtil, _ := kubeUtils.New(kubeclient, radixclient)

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, anyContainerRegistry)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient
}

func teardownTest() {
	// Celanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.DeploymentsHistoryLimitEnvironmentVariable)
}

func TestObjectSynced_MultiComponent_ContainsAllElements(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

	aRadixRegistrationBuilder := utils.ARadixRegistration().
		WithMachineUser(true)

	aRadixApplicationBuilder := utils.ARadixApplication().
		WithRadixRegistration(aRadixRegistrationBuilder)

	aRadixDeploymentBuilder := utils.ARadixDeployment().
		WithRadixApplication(aRadixApplicationBuilder).
		WithAppName("edcradix").
		WithImageTag("axmz8").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithResource(map[string]string{
					"memory": "64Mi",
					"cpu":    "250m",
				}, map[string]string{
					"memory": "128Mi",
					"cpu":    "500m",
				}).
				WithReplicas(test.IntPtr(4)),
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/radix-loadbalancer-html-redis:1igdh").
				WithName("redis").
				WithEnvironmentVariable("a_variable", "3001").
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(test.IntPtr(0)),
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/edcradix-radixquote:axmz8").
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret"}))

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, aRadixDeploymentBuilder)

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(deployments.Items), "Number of deployments wasn't as expected")
		assert.Equal(t, "app", getDeploymentByName("app", deployments).Name, "app deployment not there")
		assert.Equal(t, int32(4), *getDeploymentByName("app", deployments).Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, 11, len(getContainerByName("app", getDeploymentByName("app", deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
		assert.Equal(t, anyContainerRegistry, getEnvVariableByNameOnDeployment(defaults.ContainerRegistryEnvironmentVariable, "app", deployments))
		assert.Equal(t, dnsZone, getEnvVariableByNameOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, "app", deployments))
		assert.Equal(t, "AnyClusterName", getEnvVariableByNameOnDeployment(defaults.ClusternameEnvironmentVariable, "app", deployments))
		assert.Equal(t, "test", getEnvVariableByNameOnDeployment(defaults.EnvironmentnameEnvironmentVariable, "app", deployments))
		assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(defaults.PublicEndpointEnvironmentVariable, "app", deployments))
		assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(defaults.CanonicalEndpointEnvironmentVariable, "app", deployments))
		assert.Equal(t, "edcradix", getEnvVariableByNameOnDeployment(defaults.RadixAppEnvironmentVariable, "app", deployments))
		assert.Equal(t, "app", getEnvVariableByNameOnDeployment(defaults.RadixComponentEnvironmentVariable, "app", deployments))
		assert.Equal(t, "(8080)", getEnvVariableByNameOnDeployment(defaults.RadixPortsEnvironmentVariable, "app", deployments))
		assert.Equal(t, "(http)", getEnvVariableByNameOnDeployment(defaults.RadixPortNamesEnvironmentVariable, "app", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixCommitHashEnvironmentVariable, "app", deployments))
		assert.Equal(t, parseQuantity("128Mi"), getContainerByName("app", getDeploymentByName("app", deployments).Spec.Template.Spec.Containers).Resources.Limits["memory"])
		assert.Equal(t, parseQuantity("500m"), getContainerByName("app", getDeploymentByName("app", deployments).Spec.Template.Spec.Containers).Resources.Limits["cpu"])
		assert.Equal(t, parseQuantity("64Mi"), getContainerByName("app", getDeploymentByName("app", deployments).Spec.Template.Spec.Containers).Resources.Requests["memory"])
		assert.Equal(t, parseQuantity("250m"), getContainerByName("app", getDeploymentByName("app", deployments).Spec.Template.Spec.Containers).Resources.Requests["cpu"])
		assert.Equal(t, "redis", getDeploymentByName("redis", deployments).Name, "redis deployment not there")
		assert.Equal(t, int32(0), *getDeploymentByName("redis", deployments).Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, 10, len(getContainerByName("redis", getDeploymentByName("redis", deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
		assert.True(t, envVariableByNameExistOnDeployment("a_variable", "redis", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.ContainerRegistryEnvironmentVariable, "redis", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, "redis", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.ClusternameEnvironmentVariable, "redis", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.EnvironmentnameEnvironmentVariable, "redis", deployments))
		assert.Equal(t, "3001", getEnvVariableByNameOnDeployment("a_variable", "redis", deployments))
		assert.True(t, deploymentByNameExists("radixquote", deployments), "radixquote deployment not there")
		assert.Equal(t, int32(DefaultReplicas), *getDeploymentByName("radixquote", deployments).Spec.Replicas, "number of replicas was unexpected")
		assert.True(t, envVariableByNameExistOnDeployment(defaults.ContainerRegistryEnvironmentVariable, "radixquote", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, "radixquote", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.ClusternameEnvironmentVariable, "radixquote", deployments))
		assert.True(t, envVariableByNameExistOnDeployment(defaults.EnvironmentnameEnvironmentVariable, "radixquote", deployments))
		assert.True(t, envVariableByNameExistOnDeployment("a_secret", "radixquote", deployments))
	})

	t.Run("validate hpa", func(t *testing.T) {
		t.Parallel()
		hpas, _ := client.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(services.Items), "Number of services wasn't as expected")
		assert.True(t, serviceByNameExists("app", services), "app service not there")
		assert.True(t, serviceByNameExists("redis", services), "redis service not there")
		assert.True(t, serviceByNameExists("radixquote", services), "radixquote service not there")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(secrets.Items), "Number of secrets was not according to spec")

		componentSecretName := utils.GetComponentSecretName("radixquote")
		assert.True(t, secretByNameExists(componentSecretName, secrets), "Component secret is not as expected")

		// Exists due to external DNS, even though this is not acive cluster
		assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")
		assert.True(t, secretByNameExists("another.alias.com", secrets), "TLS certificate for second external alias is not properly defined")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate roles", func(t *testing.T) {
		t.Parallel()
		roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})

		assert.Equal(t, 2, len(roles.Items), "Number of roles was not expected")
		assert.True(t, roleByNameExists("radix-app-adm-radixquote", roles), "Expected role radix-app-adm-radixquote to be there to access secret")

		// Exists due to external DNS, even though this is not acive cluster
		assert.True(t, roleByNameExists("radix-app-adm-app", roles), "Expected role radix-app-adm-frontend to be there to access secrets for TLS certificates")
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(rolebindings.Items), "Number of rolebindings was not expected")

		assert.True(t, roleBindingByNameExists("radix-app-adm-radixquote", rolebindings), "Expected rolebinding radix-app-adm-radixquote to be there to access secret")
		assert.Equal(t, 2, len(getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects), "Number of rolebinding subjects was not as expected")
		assert.Equal(t, "edcradix-machine-user", getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects[1].Name)

		// Exists due to external DNS, even though this is not acive cluster
		assert.True(t, roleBindingByNameExists("radix-app-adm-app", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")
	})

	t.Run("validate networkpolicy", func(t *testing.T) {
		t.Parallel()
		np, _ := client.NetworkingV1().NetworkPolicies(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(np.Items), "Number of networkpolicy was not expected")
	})

	teardownTest()
}

func TestObjectSynced_MultiComponent_NonActiveCluster_ContainsOnlyClusterSpecificIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("edcradix").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com"),
			utils.NewDeployComponentBuilder().
				WithName("redis").
				WithPort("http", 6379).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Only cluster specific ingresses for the two public components should appear")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")

	appIngress := getIngressByName("app", ingresses)
	assert.Equal(t, int32(8080), appIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteIngress := getIngressByName("radixquote", ingresses)
	assert.Equal(t, int32(3000), quoteIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "radixquote", quoteIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	teardownTest()

}

func TestObjectSynced_MultiComponent_ActiveCluster_ContainsAllAliasesAndSupportingObjects(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("edcradix").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com"),
			utils.NewDeployComponentBuilder().
				WithName("redis").
				WithPort("http", 6379).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 7, len(ingresses.Items), "Number of ingresses was not according to public components, app alias and number of external aliases")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("edcradix-url-alias", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have an external alias")
	assert.Truef(t, ingressByNameExists("another.alias.com", ingresses), "App should have another external alias")
	assert.Truef(t, ingressByNameExists("app-active-cluster-url-alias", ingresses), "App should have another external alias")
	assert.Truef(t, ingressByNameExists("radixquote-active-cluster-url-alias", ingresses), "Radixquote should have had an ingress")

	appAlias := getIngressByName("edcradix-url-alias", ingresses)
	assert.Equal(t, int32(8080), appAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "true", appAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", appAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "edcradix.app.dev.radix.equinor.com", appAlias.Spec.Rules[0].Host, "App should have an external alias")

	externalAlias := getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", externalAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", externalAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "some.alias.com", externalAlias.Spec.Rules[0].Host, "App should have an external alias")

	anotherExternalAlias := getIngressByName("another.alias.com", ingresses)
	assert.Equal(t, int32(8080), anotherExternalAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", anotherExternalAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", anotherExternalAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", anotherExternalAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", anotherExternalAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "another.alias.com", anotherExternalAlias.Spec.Rules[0].Host, "App should have an external alias")

	appActiveClusterIngress := getIngressByName("app-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(8080), appActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", appActiveClusterIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appActiveClusterIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", appActiveClusterIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appActiveClusterIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteActiveClusterIngress := getIngressByName("radixquote-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(3000), quoteActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", quoteActiveClusterIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", quoteActiveClusterIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", quoteActiveClusterIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "radixquote", quoteActiveClusterIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-adm-app", roles), "Expected role radix-app-adm-app to be there to access secrets for TLS certificates")

	appAdmAppRole := getRoleByName("radix-app-adm-app", roles)
	assert.Equal(t, "secrets", appAdmAppRole.Rules[0].Resources[0], "Expected role radix-app-adm-app should be able to access secrets")
	assert.Equal(t, "some.alias.com", appAdmAppRole.Rules[0].ResourceNames[0], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "another.alias.com", appAdmAppRole.Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for second external alias")

	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")
	assert.True(t, secretByNameExists("another.alias.com", secrets), "TLS certificate for second external alias is not properly defined")

	assert.Equal(t, corev1.SecretType("kubernetes.io/tls"), getSecretByName("some.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")
	assert.Equal(t, corev1.SecretType("kubernetes.io/tls"), getSecretByName("another.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")

	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-adm-app", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")

	teardownTest()
}

func TestObjectSynced_RadixApiAndWebhook_GetsServiceAccount(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient := setupTest()

	// Test
	t.Run("app use default SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("webhook runs custom SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("radix-api runs custom SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
			WithAppName("radix-api").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	teardownTest()
}

func TestObjectSynced_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("anyimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithImage("anotherimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(deployments.Items), "Number of deployments wasn't as expected")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(services.Items), "Number of services wasn't as expected")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")

	teardownTest()
}

func TestObjectSynced_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient := setupTest()
	anyEnvironment := "test"

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("component").
				WithEnvironmentVariables(nil).
				WithSecrets(nil)))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 7, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "Should only have default environment variables")
		assert.Equal(t, defaults.ContainerRegistryEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, defaults.RadixDNSZoneEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, defaults.ClusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, anyContainerRegistry, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, dnsZone, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)
		assert.Equal(t, clusterName, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Value)
		assert.Equal(t, defaults.EnvironmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, anyEnvironment, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Value)
		assert.Equal(t, defaults.RadixAppEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Name)
		assert.Equal(t, "app", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Value)
		assert.Equal(t, defaults.RadixComponentEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Name)
		assert.Equal(t, "component", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Value)
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(secrets.Items), "Should have no secrets")
	})

	teardownTest()
}

func TestObjectSynced_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithLabel("radix-branch", "master").
		WithLabel("radix-commit", "4faca8595c5283a9d0f17a623b9255a0d9866a2e"))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")

	t.Run("validate deploy labels", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Annotations[kubeUtils.RadixBranchAnnotation])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["radix-commit"])
	})

	teardownTest()
}

func TestObjectSynced_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient := setupTest()

	// Test
	now := time.Now().UTC()
	var firstUID, secondUID types.UID

	firstUID = "fda3d224-3115-11e9-b189-06c15a8f2fbb"
	secondUID = "5a8f2fbb-3115-11e9-b189-06c1fda3d224"

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now).
		WithUID(firstUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))

	envNamespace := utils.GetEnvironmentNamespace("app1", "prod")
	deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, deployments.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, services.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, ingresses.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	time.Sleep(1 * time.Millisecond)
	// This is one second newer deployment
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("seconddeployment").
		WithCreated(now.Add(time.Second*time.Duration(1))).
		WithUID(secondUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))

	deployments, _ = client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	services, _ = client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	// Re-apply the first  This should be ignored and cause an error as it is not the latest
	rdBuilder := utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now).
		WithUID(firstUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http"))

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, rdBuilder)

	deployments, _ = client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	services, _ = client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	teardownTest()
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))

	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8081).
				WithPublicPort("http")))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	teardownTest()
}

func TestObjectUpdated_WithAppAliasRemoved_AliasIngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true)))

	// Test
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("any-app-url-alias", ingresses), "App should have had an app alias ingress")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(false)))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	teardownTest()
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(test.IntPtr(4)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(test.IntPtr(0)),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)

	// Remove components
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(test.IntPtr(0))))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(deployments.Items), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(services.Items), "Number of services wasn't as expected")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(ingresses.Items), "Number of ingresses was not according to public components")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, utils.GetComponentSecretName(componentThreeName), secrets.Items[0].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(rolebindings.Items), "Number of rolebindings was not expected")
	})

	teardownTest()
}

func TestObjectSynced_PublicToNonPublic_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Both components should be public")

	// Remove public on component 2
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("")))

	assert.NoError(t, err)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Only component 1 should be public")

	// Remove public on component 1
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("")))

	assert.NoError(t, err)

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "No component should be public")

	teardownTest()
}

func TestConstructForTargetEnvironment_PicksTheCorrectEnvironmentConfig(t *testing.T) {
	ra := utils.ARadixApplication().
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "1234").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}).
						WithReplicas(test.IntPtr(4)),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("DB_HOST", "db-dev").
						WithEnvironmentVariable("DB_PORT", "9876").
						WithResource(map[string]string{
							"memory": "32Mi",
							"cpu":    "125m",
						}, map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}).
						WithReplicas(test.IntPtr(3)))).
		BuildRA()

	var testScenarios = []struct {
		environment           string
		expectedReplicas      int
		expectedDbHost        string
		expectedDbPort        string
		expectedMemoryLimit   string
		expectedCPULimit      string
		expectedMemoryRequest string
		expectedCPURequest    string
	}{
		{"prod", 4, "db-prod", "1234", "128Mi", "500m", "64Mi", "250m"},
		{"dev", 3, "db-dev", "9876", "64Mi", "250m", "32Mi", "125m"},
	}

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: "anyImage", ImagePath: "anyImagePath"}

	for _, testcase := range testScenarios {
		t.Run(testcase.environment, func(t *testing.T) {

			rd, _ := ConstructForTargetEnvironment(ra, "anyreg", "anyjob", "anyimageTag", "anybranch", "anycommit", componentImages, testcase.environment)

			assert.Equal(t, testcase.expectedReplicas, *rd.Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
			assert.Equal(t, testcase.expectedDbHost, rd.Spec.Components[0].EnvironmentVariables["DB_HOST"])
			assert.Equal(t, testcase.expectedDbPort, rd.Spec.Components[0].EnvironmentVariables["DB_PORT"])
			assert.Equal(t, testcase.expectedMemoryLimit, rd.Spec.Components[0].Resources.Limits["memory"])
			assert.Equal(t, testcase.expectedCPULimit, rd.Spec.Components[0].Resources.Limits["cpu"])
			assert.Equal(t, testcase.expectedMemoryRequest, rd.Spec.Components[0].Resources.Requests["memory"])
			assert.Equal(t, testcase.expectedCPURequest, rd.Spec.Components[0].Resources.Requests["cpu"])
		})
	}

}

func TestObjectSynced_PublicPort_OldPublic(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"

	// New publicPort exists, old public does not exist
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(false)))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, 80, ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue())

	// New publicPort exists, old public exists (ignored)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, 80, ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue())

	// New publicPort does not exist, old public does not exist
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("").
				WithPublic(false)))

	assert.NoError(t, err)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "Component should not be public")

	// New publicPort does not exist, old public exists (used)
	rd, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	actualPortValue := ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue()
	expectedPortValue := int(rd.Spec.Components[0].Ports[0].Port)
	assert.Equal(t, expectedPortValue, actualPortValue)

	teardownTest()
}

func TestObjectUpdated_WithAllExternalAliasRemoved_ExternalAliasIngressIsCorrectlyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com")))

	// Test
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})

	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.Equal(t, 1, len(roles.Items), "Environment should have one role for TLS cert")
	assert.True(t, roleByNameExists("radix-app-adm-frontend", roles), "Expected role radix-app-adm-frontend to be there to access secrets for TLS certificates")

	assert.Equal(t, 1, len(rolebindings.Items), "Environment should have one rolebinding for TLS cert")
	assert.True(t, roleBindingByNameExists("radix-app-adm-frontend", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")

	assert.Equal(t, 1, len(secrets.Items), "Environment should have one secret for TLS cert")
	assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	secrets, _ = client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	rolebindings, _ = client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})

	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.Equal(t, 0, len(rolebindings.Items), "Role should have been removed")
	assert.Equal(t, 0, len(rolebindings.Items), "Rolebinding should have been removed")
	assert.Equal(t, 0, len(secrets.Items), "Secret should have been removed")

}

func TestObjectUpdated_WithOneExternalAliasRemovedOrModified_AllChangesPropelyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret"})))

	// Test
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress := getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	anotherExternalAliasIngress := getIngressByName("another.alias.com", ingresses)
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.GetName(), "App should have had another external alias ingress")
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), anotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("yet.another.alias.com").
				WithSecrets([]string{"a_secret"})))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress = getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	yetAnotherExternalAliasIngress := getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithDNSExternalAlias("yet.another.alias.com").
				WithSecrets([]string{"a_secret"})))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	yetAnotherExternalAliasIngress = getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for second external alias")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(roles.Items), "Role should have been removed")

}

func TestFixedAliasIngress_ActiveCluster(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient := setupTest()

	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	// Current cluster is active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, radixDeployBuilder)

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Environment should have two ingresses")
	assert.False(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, clusterName))
	assert.True(t, strings.Contains(ingresses.Items[1].Spec.Rules[0].Host, clusterName))

	// Current cluster is not active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "newClusterName")
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, radixDeployBuilder)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Environment should have one ingresses")
	assert.True(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, clusterName))

	teardownTest()
}

func TestNewDeploymentStatus(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"

	tu, client, kubeUtil, radixclient := setupTest()

	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	rd, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, radixDeployBuilder)
	assert.Equal(t, v1.DeploymentActive, rd.Status.Condition)
	assert.True(t, !rd.Status.ActiveFrom.IsZero())
	assert.True(t, rd.Status.ActiveTo.IsZero())

	time.Sleep(2 * time.Millisecond)

	radixDeployBuilder = utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	rd2, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, radixDeployBuilder)
	rd, _ = radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(rd.GetName(), metav1.GetOptions{})

	assert.Equal(t, v1.DeploymentInactive, rd.Status.Condition)
	assert.Equal(t, rd.Status.ActiveTo, rd2.Status.ActiveFrom)

	assert.Equal(t, v1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())
}

func TestObjectUpdated_RemoveOneSecret_SecretIsRemoved(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret", "another_secret", "a_third_secret"})))

	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	anyComponentSecret := secrets.Items[0]
	assert.Equal(t, utils.GetComponentSecretName(anyComponentName), anyComponentSecret.GetName(), "Component secret is not as expected")

	// Secret is initially empty but get filled with data from the API
	assert.Equal(t, []string{}, maps.GetKeysFromByteMap(anyComponentSecret.Data), "Component secret data is not as expected")

	// Will emulate that data is set from the API
	anySecretValue := "anySecretValue"
	secretData := make(map[string][]byte)
	secretData["a_secret"] = []byte(anySecretValue)
	secretData["another_secret"] = []byte(anySecretValue)
	secretData["a_third_secret"] = []byte(anySecretValue)

	anyComponentSecret.Data = secretData
	client.CoreV1().Secrets(envNamespace).Update(&anyComponentSecret)

	// Removing one secret from config and therefor from the deployment
	// should cause it to disappear
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret", "a_third_secret"})))

	secrets, _ = client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	anyComponentSecret = secrets.Items[0]
	assert.True(t, utils.ArrayEqualElements([]string{"a_secret", "a_third_secret"}, maps.GetKeysFromByteMap(anyComponentSecret.Data)), "Component secret data is not as expected")
}

func TestHistoryLimit_IsBroken_FixedAmountOfDeployments(t *testing.T) {
	anyAppName := "any-app"
	anyComponentName := "frontend"
	anyEnvironment := "dev"
	anyLimit := 3

	tu, client, kubeUtils, radixclient := setupTest()

	// Current cluster is active cluster
	os.Setenv(defaults.DeploymentsHistoryLimitEnvironmentVariable, strconv.Itoa(anyLimit))

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	applyDeploymentWithSync(tu, client, kubeUtils, radixclient,
		utils.ARadixDeployment().
			WithDeploymentName("firstdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient,
		utils.ARadixDeployment().
			WithDeploymentName("seconddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient,
		utils.ARadixDeployment().
			WithDeploymentName("thirddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient,
		utils.ARadixDeployment().
			WithDeploymentName("fourthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	deployments, _ := radixclient.RadixV1().RadixDeployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient,
		utils.ARadixDeployment().
			WithDeploymentName("fifthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	deployments, _ = radixclient.RadixV1().RadixDeployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.False(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fifthdeployment", deployments))

	teardownTest()
}

func TestObjectUpdated_WithIngressConfig_AnnotationIsPutOnIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	// Setup
	client.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressConfigurationMap,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"ingressConfiguration": testIngressConfiguration,
		},
	})

	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithIngressConfiguration("non-existing")))

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithIngressConfiguration("socket")))

	// Test
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	appAliasIngress := getIngressByName("any-app-url-alias", ingresses)
	clusterSpecificIngress := getIngressByName("frontend", ingresses)
	activeClusterIngress := getIngressByName("frontend-active-cluster-url-alias", ingresses)
	assert.Equal(t, 2, len(appAliasIngress.ObjectMeta.Annotations))
	assert.Equal(t, 2, len(clusterSpecificIngress.ObjectMeta.Annotations))
	assert.Equal(t, 2, len(activeClusterIngress.ObjectMeta.Annotations))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app-2", "dev")).List(metav1.ListOptions{})
	appAliasIngress = getIngressByName("any-app-2-url-alias", ingresses)
	clusterSpecificIngress = getIngressByName("frontend", ingresses)
	activeClusterIngress = getIngressByName("frontend-active-cluster-url-alias", ingresses)
	assert.Equal(t, 5, len(appAliasIngress.ObjectMeta.Annotations))
	assert.Equal(t, 5, len(clusterSpecificIngress.ObjectMeta.Annotations))
	assert.Equal(t, 5, len(activeClusterIngress.ObjectMeta.Annotations))

}

func TestHPAConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	minReplicas := int32(2)
	maxReplicas := int32(4)

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(0)).
				WithHorizontalScaling(&minReplicas, maxReplicas),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas)))

	assert.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate hpas", func(t *testing.T) {
		t.Parallel()
		hpas, _ := client.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(hpas.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, hpaByNameExists(componentOneName, hpas), "componentOneName horizontal pod autoscaler is there")
		assert.True(t, hpaByNameExists(componentTwoName, hpas), "componentTwoName horizontal pod autoscaler is there")
		assert.Equal(t, int32(2), *getHPAByName(componentTwoName, hpas).Spec.MinReplicas, "componentTwoName horizontal pod autoscaler is there")
	})
}

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}

func applyDeploymentWithSync(tu *test.Utils, client kube.Interface, kubeUtil *kubeUtils.Kube,
	radixclient radixclient.Interface, deploymentBuilder utils.DeploymentBuilder) (*v1.RadixDeployment, error) {
	rd, err := tu.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment, err := NewDeployment(client, kubeUtil, radixclient, nil, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return nil, err
	}

	return rd, nil
}

func applyDeploymentUpdateWithSync(tu *test.Utils, client kube.Interface, kubeUtil *kubeUtils.Kube,
	radixclient radixclient.Interface, deploymentBuilder utils.DeploymentBuilder) error {
	rd, err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment, err := NewDeployment(client, kubeUtil, radixclient, nil, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return err
	}

	return nil
}

func envVariableByNameExistOnDeployment(name, deploymentName string, deployments *extension.DeploymentList) bool {
	return envVariableByNameExist(name, getContainerByName(deploymentName, getDeploymentByName(deploymentName, deployments).Spec.Template.Spec.Containers).Env)
}

func getEnvVariableByNameOnDeployment(name, deploymentName string, deployments *extension.DeploymentList) string {
	return getEnvVariableByName(name, getContainerByName(deploymentName, getDeploymentByName(deploymentName, deployments).Spec.Template.Spec.Containers).Env)
}

func radixDeploymentByNameExists(name string, deployments *v1.RadixDeploymentList) bool {
	return getRadixDeploymentByName(name, deployments) != nil
}

func getRadixDeploymentByName(name string, deployments *v1.RadixDeploymentList) *v1.RadixDeployment {
	for _, deployment := range deployments.Items {
		if deployment.Name == name {
			return &deployment
		}
	}

	return nil
}

func deploymentByNameExists(name string, deployments *extension.DeploymentList) bool {
	return getDeploymentByName(name, deployments) != nil
}

func getDeploymentByName(name string, deployments *extension.DeploymentList) *extension.Deployment {
	for _, deployment := range deployments.Items {
		if deployment.Name == name {
			return &deployment
		}
	}

	return nil
}

func getContainerByName(name string, containers []corev1.Container) *corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return &container
		}
	}

	return nil
}

func envVariableByNameExist(name string, envVars []corev1.EnvVar) bool {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return true
		}
	}

	return false
}

func getEnvVariableByName(name string, envVars []corev1.EnvVar) string {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar.Value
		}
	}

	return ""
}

func hpaByNameExists(name string, hpas *autoscalingv1.HorizontalPodAutoscalerList) bool {
	for _, hpa := range hpas.Items {
		if hpa.Name == name {
			return true
		}
	}

	return false
}

func getHPAByName(name string, hpas *autoscalingv1.HorizontalPodAutoscalerList) *autoscalingv1.HorizontalPodAutoscaler {
	for _, hpa := range hpas.Items {
		if hpa.Name == name {
			return &hpa
		}
	}

	return nil
}

func serviceByNameExists(name string, services *corev1.ServiceList) bool {
	for _, service := range services.Items {
		if service.Name == name {
			return true
		}
	}

	return false
}

func getIngressByName(name string, ingresses *extension.IngressList) *extension.Ingress {
	for _, ingress := range ingresses.Items {
		if ingress.Name == name {
			return &ingress
		}
	}

	return nil
}

func ingressByNameExists(name string, ingresses *extension.IngressList) bool {
	ingress := getIngressByName(name, ingresses)
	if ingress != nil {
		return true
	}

	return false
}

func getRoleByName(name string, roles *rbacv1.RoleList) *rbacv1.Role {
	for _, role := range roles.Items {
		if role.Name == name {
			return &role
		}
	}

	return nil
}

func roleByNameExists(name string, roles *rbacv1.RoleList) bool {
	role := getRoleByName(name, roles)
	if role != nil {
		return true
	}

	return false
}

func getSecretByName(name string, secrets *corev1.SecretList) *corev1.Secret {
	for _, secret := range secrets.Items {
		if secret.Name == name {
			return &secret
		}
	}

	return nil
}

func secretByNameExists(name string, secrets *corev1.SecretList) bool {
	secret := getSecretByName(name, secrets)
	if secret != nil {
		return true
	}

	return false
}

func getRoleBindingByName(name string, roleBindings *rbacv1.RoleBindingList) *rbacv1.RoleBinding {
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.Name == name {
			return &roleBinding
		}
	}

	return nil
}

func roleBindingByNameExists(name string, roleBindings *rbacv1.RoleBindingList) bool {
	role := getRoleBindingByName(name, roleBindings)
	if role != nil {
		return true
	}

	return false
}
