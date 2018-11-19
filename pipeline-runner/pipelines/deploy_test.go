package onpush

import (
	"testing"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	radix "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const deployTestFilePath = "./testdata/radixconfig.variable.yaml"

func TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExtists(t *testing.T) {
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()

	rr := utils.ARadixRegistration().
		WithName("any-app").
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName("any-app").
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithPublic(true).
				WithReplicas(4).
				WithPort("http", 8080),
			utils.AnApplicationComponent().
				WithName("redis").
				WithPublic(false).
				WithPort("http", 6379).
				WithEnvironmentVariable("dev", "DB_HOST", "db-dev").
				WithEnvironmentVariable("dev", "DB_PORT", "1234").
				WithEnvironmentVariable("prod", "DB_HOST", "db-prod").
				WithEnvironmentVariable("prod", "DB_PORT", "9876").
				WithEnvironmentVariable("no-existing-env", "DB_HOST", "db-prod").
				WithEnvironmentVariable("no-existing-env", "DB_PORT", "9876")).
		BuildRA()

	cli, _ := Init(kubeclient, radixclient)
	targetEnvs := getTargetEnvironmentsAsMap("master", ra)

	rds, err := cli.Deploy(rr, ra, "anytag", "master", "4faca8595c5283a9d0f17a623b9255a0d9866a2e", targetEnvs)
	t.Run("validate deploy", func(t *testing.T) {
		assert.NoError(t, err)
		assert.True(t, len(rds) > 0)
	})

	rdNameDev := rds[0].Name
	t.Run("validate namespace creation", func(t *testing.T) {
		devNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-dev", metav1.GetOptions{})
		assert.NotNil(t, devNs)
		prodNs, _ := kubeclient.CoreV1().Namespaces().Get("any-app-prod", metav1.GetOptions{})
		assert.NotNil(t, prodNs)
	})

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
		assert.NotEmpty(t, rdDev.Labels["branch"])
		assert.NotEmpty(t, rdDev.Labels["commitID"])
		assert.Equal(t, "master", rdDev.Labels["branch"])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", rdDev.Labels["commitID"])
	})
}
