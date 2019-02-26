package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestConstructForTargetEnvironment_PicksTheCorrectEnvironmentConfig(t *testing.T) {
	ra := utils.ARadixApplication().
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironmentName("prod").
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
						WithEnvironmentName("dev").
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
