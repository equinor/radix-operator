package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

const (
	appName       = "app"
	componentName = "comp"
	// containerRegistry = "radixdev.azurecr.io"
	env = "dev"

	anyImage     = componentName
	anyImagePath = anyImage
	anyImageTag  = "latest"
)

func TestGetRadixComponentsForEnv_PublicPort_OldPublic(t *testing.T) {
	// New publicPort does not exist, old public does not exist
	ra := utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443)).BuildRA()

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	deployComponent := getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public does not exist
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublicPort("http")).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public exists (ignored)
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublicPort("http").
				WithPublic(true)).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort does not exist, old public exists (used)
	ra = utils.ARadixApplication().
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName(componentName).
				WithPort("http", 80).
				WithPort("https", 443).
				WithPublic(true)).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages)
	assert.Equal(t, ra.Spec.Components[0].Ports[0].Name, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, false, deployComponent[0].Public)
}

func TestGetRadixComponentsForEnv_ListOfExternalAliasesForComponent_GetListOfAliases(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("componentA"),
			utils.NewApplicationComponentBuilder().
				WithName("componentB")).
		WithDNSExternalAlias("some.alias.com", "prod", "componentA").
		WithDNSExternalAlias("another.alias.com", "prod", "componentA").
		WithDNSExternalAlias("athird.alias.com", "prod", "componentB").BuildRA()

	deployComponent := getRadixComponentsForEnv(ra, anyContainerRegistry, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 2, len(deployComponent[0].DNSExternalAlias))
	assert.Equal(t, "some.alias.com", deployComponent[0].DNSExternalAlias[0])
	assert.Equal(t, "another.alias.com", deployComponent[0].DNSExternalAlias[1])

	assert.Equal(t, 1, len(deployComponent[1].DNSExternalAlias))
	assert.Equal(t, "athird.alias.com", deployComponent[1].DNSExternalAlias[0])

	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, "dev", componentImages)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 0, len(deployComponent[0].DNSExternalAlias))
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_No_Override(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonEnvironmentVariable("ENV_COMMON_1", "environment_common_1").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_1", "environment_1"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_2", "environment_2")),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_3", "environment_3"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_4", "environment_4"))).BuildRA()

	deployComponentProd := getRadixComponentsForEnv(ra, anyContainerRegistry, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 2, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 2, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev := getRadixComponentsForEnv(ra, anyContainerRegistry, "dev", componentImages)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 2, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_2", deployComponentDev[0].EnvironmentVariables["ENV_2"])
	assert.Equal(t, "environment_common_1", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 2, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_4", deployComponentDev[1].EnvironmentVariables["ENV_4"])
	assert.Equal(t, "environment_common_2", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_With_Override(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonEnvironmentVariable("ENV_COMMON_1", "environment_common_1").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_1", "environment_1").
						WithEnvironmentVariable("ENV_COMMON_1", "environment_common_1_prod_override"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_2", "environment_2").
						WithEnvironmentVariable("ENV_COMMON_1", "environment_common_1_dev_override")),

			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("ENV_3", "environment_3").
						WithEnvironmentVariable("ENV_COMMON_2", "environment_common_2_prod_override"),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("ENV_4", "environment_4").
						WithEnvironmentVariable("ENV_COMMON_2", "environment_common_2_dev_override"))).BuildRA()

	deployComponentProd := getRadixComponentsForEnv(ra, anyContainerRegistry, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 2, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1_prod_override", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 2, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2_prod_override", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev := getRadixComponentsForEnv(ra, anyContainerRegistry, "dev", componentImages)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 2, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_2", deployComponentDev[0].EnvironmentVariables["ENV_2"])
	assert.Equal(t, "environment_common_1_dev_override", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 2, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_4", deployComponentDev[1].EnvironmentVariables["ENV_4"])
	assert.Equal(t, "environment_common_2_dev_override", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func getRABuilder() utils.ApplicationBuilder {
	raBuilder := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master")
	return raBuilder
}
