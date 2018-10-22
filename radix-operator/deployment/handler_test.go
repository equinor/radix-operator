package deployment

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	registration "github.com/statoil/radix-operator/radix-operator/registration"
	testutils "github.com/statoil/radix-operator/radix-operator/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestObjectCreated_NoRegistration_ReturnsError(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	testUtils.CreateClusterPrerequisites("AnyClusterName")

	err := testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithRadixApplication(utils.ARadixApplication().
			WithRadixRegistration(nil)))
	assert.Error(t, err)
}

func TestObjectCreated_MultiComponent_ContainsAllElements(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	testUtils.CreateClusterPrerequisites("AnyClusterName")

	// Test
	err := testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("edcradix").
		WithImageTag("axmz8").
		WithEnvironment("test").
		WithComponents([]utils.DeployComponentBuilder{
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
				WithReplicas(0).
				WithSecrets([]string{"a_secret"})}))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")
	deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(deployments.Items), "ObjectCreated - Number of deployments wasn't as expected")
	assert.Equal(t, "app", deployments.Items[0].Name, "ObjectCreated - app deployment not there")
	assert.Equal(t, int32(4), *deployments.Items[0].Spec.Replicas, "ObjectCreated - number of replicas was unexpected")
	assert.Equal(t, "redis", deployments.Items[1].Name, "ObjectCreated - redis deployment not there")
	assert.Equal(t, int32(1), *deployments.Items[1].Spec.Replicas, "ObjectCreated - number of replicas was unexpected")
	assert.Equal(t, 3, len(deployments.Items[1].Spec.Template.Spec.Containers[0].Env), "ObjectCreated - number of environment variables was unexpected for component. It should contain default and custom")
	assert.Equal(t, "a_variable", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[1].Name)
	assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[1].Spec.Template.Spec.Containers[0].Env[2].Name)
	assert.Equal(t, "3001", deployments.Items[1].Spec.Template.Spec.Containers[0].Env[0].Value)
	assert.Equal(t, "radixquote", deployments.Items[2].Name, "ObjectCreated - radixquote deployment not there")
	assert.Equal(t, int32(1), *deployments.Items[2].Spec.Replicas, "ObjectCreated - number of replicas was unexpected")
	assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[2].Spec.Template.Spec.Containers[0].Env[1].Name)
	assert.Equal(t, "a_secret", deployments.Items[2].Spec.Template.Spec.Containers[0].Env[2].Name)

	services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(services.Items), "ObjectCreated - Number of services wasn't as expected")
	assert.Equal(t, "app", services.Items[0].Name, "ObjectCreated - app service not there")
	assert.Equal(t, "redis", services.Items[1].Name, "ObjectCreated - redis service not there")
	assert.Equal(t, "radixquote", services.Items[2].Name, "ObjectCreated - radixquote service not there")

	ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "ObjectCreated - Number of ingresses was not according to public components")
	assert.Equal(t, "app", ingresses.Items[0].GetName(), "ObjectCreated - App should have had an ingress")
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "ObjectCreated - Port was unexpected")
	assert.Equal(t, "radixquote", ingresses.Items[1].GetName(), "ObjectCreated - Radixquote should have had an ingress")
	assert.Equal(t, int32(3000), ingresses.Items[1].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "ObjectCreated - Port was unexpected")

	secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(secrets.Items), "ObjectCreated - Number of secrets was not according to spec")
	assert.Equal(t, "radix-docker", secrets.Items[0].GetName(), "ObjectCreated - Component secret is not as expected")
	assert.Equal(t, "radixquote", secrets.Items[1].GetName(), "ObjectCreated - Component secret is not as expected")

	serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(serviceAccounts.Items), "ObjectCreated - Number of service accounts was not expected")

	rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(rolebindings.Items), "ObjectCreated - Number of rolebindings was not expected")
	assert.Equal(t, "radix-app-adm-radixquote", rolebindings.Items[0].GetName(), "ObjectCreated - Expected rolebinding radix-app-adm-radixquote to be there to access secret")
	assert.Equal(t, "radix-app-admin-envs", rolebindings.Items[1].GetName(), "ObjectCreated - Expected rolebinding radix-app-admin-envs to be there by default")
}

func TestObjectCreated_RadixApiAndWebhook_GetsServiceAccount(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	testUtils.CreateClusterPrerequisites("AnyClusterName")

	// Test
	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("any-other-app").
		WithEnvironment("test"))

	serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(serviceAccounts.Items), "ObjectCreated - Number of service accounts was not expected")

	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("radix-github-webhook").
		WithEnvironment("test"))

	serviceAccounts, _ = kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(serviceAccounts.Items), "ObjectCreated - Number of service accounts was not expected")

	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("radix-api").
		WithEnvironment("test"))

	serviceAccounts, _ = kubeclient.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(serviceAccounts.Items), "ObjectCreated - Number of service accounts was not expected")

}

func TestObjectCreated_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	testUtils.CreateClusterPrerequisites("AnyClusterName")

	// Test
	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithComponents([]utils.DeployComponentBuilder{
			utils.NewDeployComponentBuilder().
				WithImage("anyimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true),
			utils.NewDeployComponentBuilder().
				WithImage("anotherimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublic(true)}))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(deployments.Items), "ObjectCreated - Number of deployments wasn't as expected")

	services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(services.Items), "ObjectCreated - Number of services wasn't as expected")

	ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "ObjectCreated - Number of ingresses was not according to public components")
}

func TestObjectCreated_NoEnvAndNoSecrets_ContainsNoEnvVariableOrSecret(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	anyClustername := "AnyClusterName"
	anyEnvironment := "test"

	testUtils.CreateClusterPrerequisites(anyClustername)

	// Test
	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment(anyEnvironment).
		WithComponents([]utils.DeployComponentBuilder{
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithEnvironmentVariables(nil).
				WithSecrets(nil)}))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := kubeclient.ExtensionsV1beta1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "ObjectCreated - Should only have default environment variables")
	assert.Equal(t, clusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, anyClustername, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
	assert.Equal(t, environmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
	assert.Equal(t, anyEnvironment, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)

	secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(secrets.Items), "ObjectCreated - Should only have default secret")
}
