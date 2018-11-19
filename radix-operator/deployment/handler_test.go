package deployment

import (
	"fmt"
	"testing"
	"time"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	registration "github.com/statoil/radix-operator/radix-operator/registration"
	"github.com/statoil/radix-operator/radix-operator/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"

func setupTest() (*test.Utils, kube.Interface) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	handlerTestUtils := test.NewHandlerTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	handlerTestUtils.CreateClusterPrerequisites(clusterName)
	return &handlerTestUtils, kubeclient
}

func TestObjectCreated_NoRegistration_ReturnsError(t *testing.T) {
	handlerTestUtils, _ := setupTest()

	err := handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithRadixApplication(utils.ARadixApplication().
			WithRadixRegistration(nil)))
	assert.Error(t, err)
}

func TestObjectCreated_MultiComponent_ContainsAllElements(t *testing.T) {
	handlerTestUtils, kubeclient := setupTest()

	// Test
	err := handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("edcradix").
		WithImageTag("axmz8").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true).
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
		deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(deployments.Items), "Number of deployments wasn't as expected")
		assert.Equal(t, "app", deployments.Items[0].Name, "app deployment not there")
		assert.Equal(t, int32(4), *deployments.Items[0].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, "redis", deployments.Items[1].Name, "redis deployment not there")
		assert.Equal(t, int32(2), *deployments.Items[1].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, 3, len(deployments.Items[1].Spec.Template.Spec.Containers[0].Env), "number of environment variables was unexpected for component. It should contain default and custom")
		assert.Equal(t, "a_variable", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, "3001", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, "radixquote", deployments.Items[2].Name, "radixquote deployment not there")
		assert.Equal(t, int32(2), *deployments.Items[2].Spec.Replicas, "number of replicas was unexpected")
		assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, "a_secret", deployments.Items[2].Spec.Template.Spec.Containers[0].Env[2].Name)
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 3, len(services.Items), "Number of services wasn't as expected")
		assert.Equal(t, "app", services.Items[0].Name, "app service not there")
		assert.Equal(t, "redis", services.Items[1].Name, "redis service not there")
		assert.Equal(t, "radixquote", services.Items[2].Name, "radixquote service not there")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(ingresses.Items), "Number of ingresses was not according to public components")
		assert.Equal(t, "app", ingresses.Items[0].GetName(), "App should have had an ingress")
		assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
		assert.Equal(t, "radixquote", ingresses.Items[1].GetName(), "Radixquote should have had an ingress")
		assert.Equal(t, int32(3000), ingresses.Items[1].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, "radix-docker", secrets.Items[0].GetName(), "Component secret is not as expected")
		assert.Equal(t, "radixquote", secrets.Items[1].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(rolebindings.Items), "Number of rolebindings was not expected")
		assert.Equal(t, "radix-app-adm-radixquote", rolebindings.Items[0].GetName(), "Expected rolebinding radix-app-adm-radixquote to be there to access secret")
		assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[1].GetName(), "Expected rolebinding radix-app-admin-envs to be there by default")
	})
}

func TestObjectCreated_RadixApiAndWebhook_GetsServiceAccount(t *testing.T) {
	// Setup
	handlerTestUtils, kubeclient := setupTest()

	// Test
	t.Run("app use default SA", func(t *testing.T) {
		handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("webhook runs custom SA", func(t *testing.T) {
		handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))

		serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("radix-api runs custom SA", func(t *testing.T) {
		handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
			WithAppName("radix-api").
			WithEnvironment("test"))

		serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})
}

func TestObjectCreated_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	handlerTestUtils, kubeclient := setupTest()

	// Test
	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
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
	deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(deployments.Items), "Number of deployments wasn't as expected")

	services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(services.Items), "Number of services wasn't as expected")

	ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")
}

func TestObjectCreated_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	handlerTestUtils, kubeclient := setupTest()
	anyEnvironment := "test"

	// Test
	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithEnvironmentVariables(nil).
				WithSecrets(nil)))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "Should only have default environment variables")
		assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, clusterName, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, anyEnvironment, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Should only have default secret")
	})
}

func TestObjectCreated_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	handlerTestUtils, kubeclient := setupTest()

	// Test
	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithLabel("branch", "master").
		WithLabel("commitID", "4faca8595c5283a9d0f17a623b9255a0d9866a2e"))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")

	t.Run("validate deploy labels", func(t *testing.T) {
		t.Parallel()
		deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Labels["branch"])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["commitID"])
	})

}

func TestObjectUpdated_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	handlerTestUtils, _ := setupTest()

	// Test
	now := time.Now()

	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now))

	// This is one second newer deployment
	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("seconddeployment").
		WithCreated(now.Add(time.Second * time.Duration(1))))

	// Re-apply the first deployment. This should be ignored and cause an error as it is not the latest
	rdBuilder := utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now)
	err := handlerTestUtils.ApplyDeploymentUpdate(rdBuilder)

	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("RadixDeployment %s was not the latest. Ignoring", rdBuilder.BuildRD().Name), err.Error())
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled(t *testing.T) {
	handlerTestUtils, kubeclient := setupTest()

	// Test
	handlerTestUtils.ApplyDeployment(utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true)))

	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")
	ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	handlerTestUtils.ApplyDeploymentUpdate(utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8081).
				WithPublic(true)))

	ingresses, _ = kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
}
