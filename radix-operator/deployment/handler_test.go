package deployment

import (
	"path/filepath"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	registration "github.com/statoil/radix-operator/radix-operator/registration"
	testutils "github.com/statoil/radix-operator/radix-operator/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestObjectCreated_(t *testing.T) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deploymentHandler := NewDeployHandler(kubeclient, radixclient)

	testUtils := testutils.NewTestUtils(kubeclient, radixclient, &registrationHandler, &deploymentHandler)
	testUtils.CreateClusterPrerequisites()

	// Test
	testUtils.ApplyDeployment(utils.ARadixDeployment().
		WithAppName("edcradix").
		WithImageTag("axmz8").
		WithEnvironment("test").
		WithComponent(utils.NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
			WithName("app").
			WithPort("http", 8080).
			WithPublic(true).
			WithReplicas(4)).
		WithComponent(utils.NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/radix-loadbalancer-html-redis:1igdh").
			WithName("redis").
			WithEnvironmentVariable("a_variable", "3001").
			WithPort("http", 6379).
			WithPublic(false).
			WithReplicas(0)).
		WithComponent(utils.NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/edcradix-radixquote:axmz8").
			WithName("radixquote").
			WithPort("http", 3000).
			WithPublic(true).
			WithReplicas(0).
			WithSecrets([]string{"a_secret"})))

	envNamespace := testutils.GetNamespaceForApplicationEnvironment("edcradix", "test")
	services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(services.Items), "ObjectCreated - Number of components was not handled correctly")
	assert.Equal(t, "app", services.Items[0].Name, "ObjectCreated - app service not there")
	assert.Equal(t, "redis", services.Items[1].Name, "ObjectCreated - redis service not there")
	assert.Equal(t, "radixquote", services.Items[2].Name, "ObjectCreated - radixquote service not there")

	ingresses, _ := kubeclient.ExtensionsV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "ObjectCreated - Number of ingresses was not according to public components")

	// Teardown
	t.Log("Teardown")

}

func Test_map_rd_to_deploy_name(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[0]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, deploy.Name, component.Name)
}

func Test_map_rd_to_deploy_replica(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[0]
	deploy := getDeploymentConfig(rd, component)
	replicas := *deploy.Spec.Replicas
	assert.Equal(t, int32(4), replicas)
}

func Test_map_rd_to_deploy_variables(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[1]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, "a_variable", deploy.Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, "3001", deploy.Spec.Template.Spec.Containers[0].Env[0].Value)
}

func Test_map_rd_to_deploy_secrets(t *testing.T) {
	file, _ := filepath.Abs("./testdata/radixdeploy.yaml")
	rd, _ := utils.GetRadixDeploy(file)

	component := rd.Spec.Components[2]
	deploy := getDeploymentConfig(rd, component)
	assert.Equal(t, "a_secret", deploy.Spec.Template.Spec.Containers[0].Env[0].Name)
}
