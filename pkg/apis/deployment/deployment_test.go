package deployment

import (
	"os"
	"testing"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const containerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface, radixclient.Interface) {
	// Setup
	os.Setenv(OperatorDNSZoneEnvironmentVariable, dnsZone)
	os.Setenv(OperatorAppAliasBaseURLEnvironmentVariable, ".app.dev.radix.equinor.com")

	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, kubeclient, radixclient
}

func TestObjectSynced_MultiComponent_ContainsAllElements(t *testing.T) {
	tu, client, radixclient := setupTest()

	// Test
	_, err := applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName("edcradix").
		WithImageTag("axmz8").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true).
				WithDNSAppAlias(true).
				WithResource(map[string]string{
					"memory": "64Mi",
					"cpu":    "250m",
				}, map[string]string{
					"memory": "128Mi",
					"cpu":    "500m",
				}).
				WithReplicas(4),
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/radix-loadbalancer-html-redis:1igdh").
				WithName("redis").
				WithEnvironmentVariable("a_variable", "3001").
				WithPort("http", 6379).
				WithPublic(false).
				WithReplicas(0),
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/edcradix-radixquote:axmz8").
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublic(true).
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(deployments.Items), "Number of deployments wasn't as expected")
		assert.Equal(t, "app", deployments.Items[0].Name, "app deployment not there")
		assert.Equal(t, int32(4), *deployments.Items[0].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, 9, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "number of environment variables was unexpected for component. It should contain default and custom")
		assert.Equal(t, ContainerRegistryEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, containerRegistry, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, RadixDNSZoneEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, dnsZone, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)
		assert.Equal(t, ClusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, "AnyClusterName", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Value)
		assert.Equal(t, EnvironmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, "test", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Value)
		assert.Equal(t, PublicEndpointEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Name)
		assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Value)
		assert.Equal(t, RadixAppEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Name)
		assert.Equal(t, "edcradix", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Value)
		assert.Equal(t, RadixComponentEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[6].Name)
		assert.Equal(t, "app", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[6].Value)
		assert.Equal(t, RadixPortsEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[7].Name)
		assert.Equal(t, "(8080)", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[7].Value)
		assert.Equal(t, RadixPortNamesEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[8].Name)
		assert.Equal(t, "(http)", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[8].Value)
		assert.Equal(t, parseQuantity("128Mi"), deployments.Items[0].Spec.Template.Spec.Containers[0].Resources.Limits["memory"])
		assert.Equal(t, parseQuantity("500m"), deployments.Items[0].Spec.Template.Spec.Containers[0].Resources.Limits["cpu"])
		assert.Equal(t, parseQuantity("64Mi"), deployments.Items[0].Spec.Template.Spec.Containers[0].Resources.Requests["memory"])
		assert.Equal(t, parseQuantity("250m"), deployments.Items[0].Spec.Template.Spec.Containers[0].Resources.Requests["cpu"])
		assert.Equal(t, "redis", deployments.Items[1].Name, "redis deployment not there")
		assert.Equal(t, int32(DefaultReplicas), *deployments.Items[1].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, 9, len(deployments.Items[1].Spec.Template.Spec.Containers[0].Env), "number of environment variables was unexpected for component. It should contain default and custom")
		assert.Equal(t, "a_variable", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, ContainerRegistryEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, RadixDNSZoneEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, ClusternameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, EnvironmentnameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[4].Name)
		assert.Equal(t, "3001", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, "radixquote", deployments.Items[2].Name, "radixquote deployment not there")
		assert.Equal(t, int32(DefaultReplicas), *deployments.Items[2].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, ContainerRegistryEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, RadixDNSZoneEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, ClusternameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, EnvironmentnameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, "a_secret", deployments.Items[2].Spec.Template.Spec.Containers[0].Env[9].Name)
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(services.Items), "Number of services wasn't as expected")
		assert.Equal(t, "app", services.Items[0].Name, "app service not there")
		assert.Equal(t, "redis", services.Items[1].Name, "redis service not there")
		assert.Equal(t, "radixquote", services.Items[2].Name, "radixquote service not there")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(ingresses.Items), "Number of ingresses was not according to public components")
		assert.Equal(t, "edcradix-url-alias", ingresses.Items[0].GetName(), "App should have had an app alias ingress")
		assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
		assert.Equal(t, "true", ingresses.Items[0].Labels["radix-app-alias"], "Ingress should be an app alias")
		assert.Equal(t, "app", ingresses.Items[0].Labels["radix-component"], "Ingress should have the corresponding component")
		assert.Equal(t, "app", ingresses.Items[1].GetName(), "App should have had an ingress")
		assert.Equal(t, int32(8080), ingresses.Items[1].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
		assert.Equal(t, "false", ingresses.Items[1].Labels["radix-app-alias"], "Ingress should not be an app alias")
		assert.Equal(t, "app", ingresses.Items[1].Labels["radix-component"], "Ingress should have the corresponding component")
		assert.Equal(t, "radixquote", ingresses.Items[2].GetName(), "Radixquote should have had an ingress")
		assert.Equal(t, int32(3000), ingresses.Items[2].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
		assert.Equal(t, "false", ingresses.Items[2].Labels["radix-app-alias"], "Ingress should not be an app alias")
		assert.Equal(t, "radixquote", ingresses.Items[2].Labels["radix-component"], "Ingress should have the corresponding component")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		componentSecretName := utils.GetComponentSecretName("radixquote")
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, "radix-docker", secrets.Items[0].GetName(), "Component secret is not as expected")
		assert.Equal(t, componentSecretName, secrets.Items[1].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(rolebindings.Items), "Number of rolebindings was not expected")
		//assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")
		assert.Equal(t, "radix-app-adm-radixquote", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-adm-radixquote to be there to access secret")
	})
}

func TestObjectSynced_RadixApiAndWebhook_GetsServiceAccount(t *testing.T) {
	// Setup
	tu, client, radixclient := setupTest()

	// Test
	t.Run("app use default SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("webhook runs custom SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("radix-api runs custom SA", func(t *testing.T) {
		applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
			WithAppName("radix-api").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})
}

func TestObjectSynced_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	tu, client, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("anyimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true),
			utils.NewDeployComponentBuilder().
				WithImage("anotherimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true)))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(deployments.Items), "Number of deployments wasn't as expected")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(services.Items), "Number of services wasn't as expected")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")
}

func TestObjectSynced_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	tu, client, radixclient := setupTest()
	anyEnvironment := "test"

	// Test
	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
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
		assert.Equal(t, 6, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "Should only have default environment variables")
		assert.Equal(t, ContainerRegistryEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, RadixDNSZoneEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, ClusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, containerRegistry, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, dnsZone, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)
		assert.Equal(t, clusterName, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Value)
		assert.Equal(t, EnvironmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, anyEnvironment, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Value)
		assert.Equal(t, RadixAppEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Name)
		assert.Equal(t, "app", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Value)
		assert.Equal(t, RadixComponentEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Name)
		assert.Equal(t, "component", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Value)
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Should only have default secret")
	})
}

func TestObjectSynced_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	tu, client, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithLabel("radix-branch", "master").
		WithLabel("radix-commit", "4faca8595c5283a9d0f17a623b9255a0d9866a2e"))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")

	t.Run("validate deploy labels", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Labels["radix-branch"])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["radix-commit"])
	})

}

func TestObjectSynced_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	tu, client, radixclient := setupTest()

	// Test
	now := time.Now()
	var firstUID, secondUID types.UID

	firstUID = "fda3d224-3115-11e9-b189-06c15a8f2fbb"
	secondUID = "5a8f2fbb-3115-11e9-b189-06c1fda3d224"

	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
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
				WithPublic(true)))

	envNamespace := utils.GetEnvironmentNamespace("app1", "prod")
	deployments, _ := client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, deployments.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, services.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, ingresses.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	// This is one second newer deployment
	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("seconddeployment").
		WithCreated(now.Add(time.Second*time.Duration(1))).
		WithUID(secondUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true)))

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
				WithPublic(true))

	applyDeploymentUpdateWithSync(tu, client, radixclient, rdBuilder)

	deployments, _ = client.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	services, _ = client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, radixclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true)))

	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	applyDeploymentUpdateWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8081).
				WithPublic(true)))

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesChange(t *testing.T) {
	tu, client, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublic(true).
				WithDNSAppAlias(true).
				WithReplicas(4),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublic(false).
				WithReplicas(0),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublic(true).
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)

	// Remove components
	_, err = applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublic(false).
				WithReplicas(0)))

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
		assert.Equal(t, "radix-docker", secrets.Items[0].GetName(), "Component secret is not as expected")
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
}

func TestObjectSynced_PublicToNonPublic_HandlesChange(t *testing.T) {
	tu, client, radixclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublic(true),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublic(true)))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Both components should be public")

	// Remove public on component 2
	_, err = applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublic(true),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublic(false)))

	assert.NoError(t, err)
	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Only component 1 should be public")

	// Remove public on component 1
	_, err = applyDeploymentWithSync(tu, client, radixclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublic(false),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublic(false)))

	assert.NoError(t, err)

	ingresses, _ = client.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "No component should be public")

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
						WithReplicas(4),
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
						WithReplicas(3))).
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

	for _, testcase := range testScenarios {
		t.Run(testcase.environment, func(t *testing.T) {
			targetEnvs := make(map[string]bool)
			targetEnvs[testcase.environment] = true
			rd, _ := ConstructForTargetEnvironments(ra, "anyreg", "anyjob", "anyimage", "anybranch", "anycommit", targetEnvs)

			assert.Equal(t, testcase.expectedReplicas, rd[0].Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
			assert.Equal(t, testcase.expectedDbHost, rd[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
			assert.Equal(t, testcase.expectedDbPort, rd[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
			assert.Equal(t, testcase.expectedMemoryLimit, rd[0].Spec.Components[0].Resources.Limits["memory"])
			assert.Equal(t, testcase.expectedCPULimit, rd[0].Spec.Components[0].Resources.Limits["cpu"])
			assert.Equal(t, testcase.expectedMemoryRequest, rd[0].Spec.Components[0].Resources.Requests["memory"])
			assert.Equal(t, testcase.expectedCPURequest, rd[0].Spec.Components[0].Resources.Requests["cpu"])
		})
	}
}

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}

func applyDeploymentWithSync(tu *test.Utils, client kube.Interface,
	radixclient radixclient.Interface, deploymentBuilder utils.DeploymentBuilder) (*v1.RadixDeployment, error) {
	rd, err := tu.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment, err := NewDeployment(client, radixclient, nil, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return nil, err
	}

	return rd, nil
}

func applyDeploymentUpdateWithSync(tu *test.Utils, client kube.Interface,
	radixclient radixclient.Interface, deploymentBuilder utils.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()

	err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment, err := NewDeployment(client, radixclient, nil, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return err
	}

	return nil
}
