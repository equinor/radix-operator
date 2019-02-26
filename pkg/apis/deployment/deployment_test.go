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

	targetEnvs := make(map[string]bool)
	targetEnvs["prod"] = true
	rd, _ := ConstructForTargetEnvironments(ra, "anyreg", "anyjob", "anyimage", "anybranch", "anycommit", targetEnvs)

	assert.Equal(t, 4, rd[0].Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
	assert.Equal(t, "db-prod", rd[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
	assert.Equal(t, "1234", rd[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
	assert.Equal(t, "128Mi", rd[0].Spec.Components[0].Resources.Limits["memory"])
	assert.Equal(t, "500m", rd[0].Spec.Components[0].Resources.Limits["cpu"])
	assert.Equal(t, "64Mi", rd[0].Spec.Components[0].Resources.Requests["memory"])
	assert.Equal(t, "250m", rd[0].Spec.Components[0].Resources.Requests["cpu"])

	targetEnvs = make(map[string]bool)
	targetEnvs["dev"] = true
	rd, _ = ConstructForTargetEnvironments(ra, "anyreg", "anyjob", "anyimage", "anybranch", "anycommit", targetEnvs)

	assert.Equal(t, 3, rd[0].Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
	assert.Equal(t, "db-dev", rd[0].Spec.Components[0].EnvironmentVariables["DB_HOST"])
	assert.Equal(t, "9876", rd[0].Spec.Components[0].EnvironmentVariables["DB_PORT"])
	assert.Equal(t, "64Mi", rd[0].Spec.Components[0].Resources.Limits["memory"])
	assert.Equal(t, "250m", rd[0].Spec.Components[0].Resources.Limits["cpu"])
	assert.Equal(t, "32Mi", rd[0].Spec.Components[0].Resources.Requests["memory"])
	assert.Equal(t, "125m", rd[0].Spec.Components[0].Resources.Requests["cpu"])
}

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}
