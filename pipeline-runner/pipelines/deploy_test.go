package onpush

import (
	"fmt"
	"testing"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const (
	deployTestFilePath = "./testdata/radixconfig.variable.yaml"
	clusterName        = "AnyClusterName"
	containerRegistry  = "any.container.registry"
)

func setupTest() (*kubernetes.Clientset, *radix.Clientset) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	testUtils := commonTest.NewTestUtils(kubeclient, radixclient)
	testUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return kubeclient, radixclient
}

func TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExtists(t *testing.T) {
	kubeclient, radixclient := setupTest()

	rr := utils.ARadixRegistration().
		WithName("any-app").
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithDNSAppAlias("dev", "app").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithPublic(true).
				WithPort("http", 8080).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithReplicas(4),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithReplicas(4)),
			utils.AnApplicationComponent().
				WithName("redis").
				WithPublic(false).
				WithPort("http", 6379).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("DB_HOST", "db-dev").
						WithEnvironmentVariable("DB_PORT", "1234").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("no-existing-env").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876"))).
		BuildRA()

	// Prometheus doesn´t contain any fake
	cli, _ := Init(kubeclient, radixclient, &monitoring.Clientset{})

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, radixclient, rr, ra)
	_, targetEnvs := applicationConfig.IsBranchMappedToEnvironment("master")

	rds, err := cli.Deploy("any-job-name", rr, ra, "anytag", "master", "4faca8595c5283a9d0f17a623b9255a0d9866a2e", targetEnvs)
	t.Run("validate deploy", func(t *testing.T) {
		assert.NoError(t, err)
		assert.True(t, len(rds) > 0)
	})

	rdNameDev := rds[0].Name

	t.Run("validate deployment exist in only the namespace of the modified branch", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(rdNameDev, metav1.GetOptions{})
		assert.NotNil(t, rdDev)

		rdProd, _ := radixclient.RadixV1().RadixDeployments("any-app-prod").Get(rdNameDev, metav1.GetOptions{})
		assert.Nil(t, rdProd)
	})

	t.Run("validate deployment environment variables", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(rdNameDev, metav1.GetOptions{})
		assert.Equal(t, 2, len(rdDev.Spec.Components))
		assert.Equal(t, 2, len(rdDev.Spec.Components[1].EnvironmentVariables))
		assert.Equal(t, "db-dev", rdDev.Spec.Components[1].EnvironmentVariables["DB_HOST"])
		assert.Equal(t, "1234", rdDev.Spec.Components[1].EnvironmentVariables["DB_PORT"])
		assert.NotEmpty(t, rdDev.Labels["radix-branch"])
		assert.NotEmpty(t, rdDev.Labels["radix-commit"])
		assert.NotEmpty(t, rdDev.Labels["radix-job-name"])
		assert.Equal(t, "master", rdDev.Labels["radix-branch"])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", rdDev.Labels["radix-commit"])
		assert.Equal(t, "any-job-name", rdDev.Labels["radix-job-name"])
	})

	t.Run("validate dns app alias", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(rdNameDev, metav1.GetOptions{})
		assert.True(t, rdDev.Spec.Components[0].DNSAppAlias)
		assert.False(t, rdDev.Spec.Components[1].DNSAppAlias)
	})

	t.Run("validate resources", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(rdNameDev, metav1.GetOptions{})

		fmt.Print(rdDev.Spec.Components[0].Resources)
		fmt.Print(rdDev.Spec.Components[1].Resources)
		assert.NotNil(t, rdDev.Spec.Components[1].Resources)
		assert.Equal(t, "128Mi", rdDev.Spec.Components[1].Resources.Limits["memory"])
		assert.Equal(t, "500m", rdDev.Spec.Components[1].Resources.Limits["cpu"])
	})

}
