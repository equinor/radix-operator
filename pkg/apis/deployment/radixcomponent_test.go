package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
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

	componentImages := make(map[string]model.ComponentImage)
	componentImages["app"] = model.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	deployComponent := getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages, anyImageTag)
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
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages, anyImageTag)
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
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages, anyImageTag)
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
	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, env, componentImages, anyImageTag)
	assert.Equal(t, ra.Spec.Components[0].Ports[0].Name, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, false, deployComponent[0].Public)
}

func TestGetRadixComponentsForEnv_ListOfExternalAliasesForComponent_GetListOfAliases(t *testing.T) {
	componentImages := make(map[string]model.ComponentImage)
	componentImages["app"] = model.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

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

	deployComponent := getRadixComponentsForEnv(ra, anyContainerRegistry, "prod", componentImages, anyImageTag)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 2, len(deployComponent[0].DNSExternalAlias))
	assert.Equal(t, "some.alias.com", deployComponent[0].DNSExternalAlias[0])
	assert.Equal(t, "another.alias.com", deployComponent[0].DNSExternalAlias[1])

	assert.Equal(t, 1, len(deployComponent[1].DNSExternalAlias))
	assert.Equal(t, "athird.alias.com", deployComponent[1].DNSExternalAlias[0])

	deployComponent = getRadixComponentsForEnv(ra, anyContainerRegistry, "dev", componentImages, anyImageTag)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 0, len(deployComponent[0].DNSExternalAlias))
}

func getRABuilder() utils.ApplicationBuilder {
	raBuilder := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master")
	return raBuilder
}
