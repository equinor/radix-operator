package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

const (
	appName       = "app"
	componentName = "comp"
	// containerRegistry = "radixdev.azurecr.io"
	env      = "dev"
	imageTag = "latest"
)

func TestGetRadixComponentsForEnv_PublicPort_OldPublic(t *testing.T) {
	// New publicPort does not exist, old public does not exist
	raBuilder := getRABuilder()
	compBuilder := utils.NewApplicationComponentBuilder().
		WithName(componentName).
		WithPort("http", 80).
		WithPort("https", 443)
	ra := raBuilder.WithComponent(compBuilder).BuildRA()
	deployComponent := getRadixComponentsForEnv(ra, containerRegistry, env, imageTag)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public does not exist
	raBuilder = getRABuilder()
	compBuilder = utils.NewApplicationComponentBuilder().
		WithName(componentName).
		WithPort("http", 80).
		WithPort("https", 443).
		WithPublicPort("http")
	ra = raBuilder.WithComponent(compBuilder).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, containerRegistry, env, imageTag)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.Equal(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort exists, old public exists (ignored)
	raBuilder = getRABuilder()
	compBuilder = utils.NewApplicationComponentBuilder().
		WithName(componentName).
		WithPort("http", 80).
		WithPort("https", 443).
		WithPublicPort("http").
		WithPublic(true)
	ra = raBuilder.WithComponent(compBuilder).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, containerRegistry, env, imageTag)
	assert.Equal(t, ra.Spec.Components[0].PublicPort, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, "http", deployComponent[0].PublicPort)
	assert.Equal(t, false, deployComponent[0].Public)

	// New publicPort does not exist, old public exists (used)
	raBuilder = getRABuilder()
	compBuilder = utils.NewApplicationComponentBuilder().
		WithName(componentName).
		WithPort("http", 80).
		WithPort("https", 443).
		WithPublic(true)
	ra = raBuilder.WithComponent(compBuilder).BuildRA()
	deployComponent = getRadixComponentsForEnv(ra, containerRegistry, env, imageTag)
	assert.Equal(t, ra.Spec.Components[0].Ports[0].Name, deployComponent[0].PublicPort)
	assert.NotEqual(t, ra.Spec.Components[0].Public, deployComponent[0].Public)
	assert.Equal(t, false, deployComponent[0].Public)
}

func getRABuilder() utils.ApplicationBuilder {
	raBuilder := utils.NewRadixApplicationBuilder().
		WithAppName(appName).
		WithEnvironment("dev", "master")
	return raBuilder
}
