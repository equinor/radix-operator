package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHpa_DefaultConfigurationDoesNotHaveMemoryScaling(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest()

	raBuilder := utils.ARadixApplication().
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithAlwaysPullImageOnDeploy(true).
						WithHorizontalScaling(numbers.Int32Ptr(1), 3, numbers.Int32Ptr(68), nil)))

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: "anyImage", ImagePath: "anyImagePath"}

	envVarsMap := make(v1.EnvVarsMap)

	rd, err := ConstructForTargetEnvironment(raBuilder.BuildRA(), "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap)
	require.NoError(t, err)
	// TODO: check that the hpa does not have memory scaling in rd with overridden values

	rdBuilder := utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithEnvironment("dev").
		WithEnvironment("prod").
		WithHorizontalScaling(numbers.Int32Ptr(1), 3, numbers.Int32Ptr(68), nil)

	rd, err := ConstructForTargetEnvironment(ra, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap)

	require.NoError(t, err)
	_, err = applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient)
	require.NoError(t, err)

}
