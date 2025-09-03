package environment

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
)

func Test_getAddedOrDroppedEnvironmentNames(t *testing.T) {
	anyAppName := "test-app"

	oldRa := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("prod", "").
		WithEnvironment("qa", "main").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("doc").
				WithSourceFolder(".").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithReplicas(numbers.IntPtr(2)).
						WithResource(
							map[string]string{
								"memory": "100Mi",
								"cpu":    "100m"},
							map[string]string{
								"memory": "150Mi",
								"cpu":    "120m"},
						),
				),
		).BuildRA()

	newRa := oldRa.DeepCopy()
	newRa.Spec.Environments = []v1.Environment{
		{
			Name:  "prod",
			Build: v1.EnvBuild{},
		},
	}

	environmentsToResync := getAddedOrDroppedEnvironmentNames(oldRa, newRa)
	assert.Len(t, environmentsToResync, 1)
	assert.Equal(t, environmentsToResync[0], "qa")
}
