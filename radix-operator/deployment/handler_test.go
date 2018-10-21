package deployment

import (
	"path/filepath"
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	utils "github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	registration "github.com/statoil/radix-operator/radix-operator/registration"
	testutils "github.com/statoil/radix-operator/radix-operator/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestObjectCreated_(t *testing.T) {
	// Setup
	logger.Error("Setup")
	stop := make(chan struct{})
	defer close(stop)

	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testutils.CreateClusterPrerequisites(kubeclient)

	registrationHandler := registration.NewRegistrationHandler(kubeclient)
	deployHandler := NewDeployHandler(kubeclient, radixclient)

	anyRadixDeployment := utils.ARadixDeployment().
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
			WithSecrets([]string{"a_secret"}))

	assert.NoError(t, err, "ObjectCreated - Unexpected error")

	services, err := kubeclient.CoreV1().Services(getNamespaceForApplicationEnvironment("edcradix", "test")).List(metav1.ListOptions{})
	assert.NoError(t, err, "ObjectCreated - Unexpected error")

	serviceName := services.Items[1].GetName()
	logger.Infof("%s", serviceName)

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
