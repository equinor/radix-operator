package deployment

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

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

func TestGetAuthenticationForComponent(t *testing.T) {
	var x0 *v1.Authentication
	x0 = nil

	verificationOpt := v1.VerificationTypeOptionalNoCa
	x1 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}
	x2 := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			Verification:              &verificationOpt,
			PassCertificateToUpstream: utils.BoolPtr(false),
		},
	}
	expected := &v1.Authentication{
		ClientCertificate: &v1.ClientCertificate{
			Verification:              &verificationOpt,
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}

	auth1 := GetAuthenticationForComponent(x1, x2)
	assert.Equal(t, expected, auth1)

	auth2 := GetAuthenticationForComponent(x1, nil)
	assert.Equal(t, x1, auth2)

	auth3 := GetAuthenticationForComponent(nil, x2)
	assert.Equal(t, x2, auth3)

	auth4 := GetAuthenticationForComponent(nil, nil)
	assert.Equal(t, x0, auth4)
}

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

	deployComponent := getRadixComponentsForEnv(ra, env, componentImages)
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
	deployComponent = getRadixComponentsForEnv(ra, env, componentImages)
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
	deployComponent = getRadixComponentsForEnv(ra, env, componentImages)
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
	deployComponent = getRadixComponentsForEnv(ra, env, componentImages)
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

	deployComponent := getRadixComponentsForEnv(ra, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponent))
	assert.Equal(t, 2, len(deployComponent[0].DNSExternalAlias))
	assert.Equal(t, "some.alias.com", deployComponent[0].DNSExternalAlias[0])
	assert.Equal(t, "another.alias.com", deployComponent[0].DNSExternalAlias[1])

	assert.Equal(t, 1, len(deployComponent[1].DNSExternalAlias))
	assert.Equal(t, "athird.alias.com", deployComponent[1].DNSExternalAlias[0])

	deployComponent = getRadixComponentsForEnv(ra, "dev", componentImages)
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

	deployComponentProd := getRadixComponentsForEnv(ra, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 2, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 2, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev := getRadixComponentsForEnv(ra, "dev", componentImages)
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

	deployComponentProd := getRadixComponentsForEnv(ra, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 2, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_1", deployComponentProd[0].EnvironmentVariables["ENV_1"])
	assert.Equal(t, "environment_common_1_prod_override", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 2, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_3", deployComponentProd[1].EnvironmentVariables["ENV_3"])
	assert.Equal(t, "environment_common_2_prod_override", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev := getRadixComponentsForEnv(ra, "dev", componentImages)
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

func TestGetRadixComponentsForEnv_CommonEnvironmentVariables_NilVariablesMapInEnvConfig(t *testing.T) {
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
						WithNilVariablesMap(),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithNilVariablesMap()),
			utils.NewApplicationComponentBuilder().
				WithName("comp_2").
				WithCommonEnvironmentVariable("ENV_COMMON_2", "environment_common_2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithNilVariablesMap(),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithNilVariablesMap())).BuildRA()

	deployComponentProd := getRadixComponentsForEnv(ra, "prod", componentImages)
	assert.Equal(t, 2, len(deployComponentProd))

	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, 1, len(deployComponentProd[0].EnvironmentVariables))
	assert.Equal(t, "environment_common_1", deployComponentProd[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentProd[1].Name)
	assert.Equal(t, 1, len(deployComponentProd[1].EnvironmentVariables))
	assert.Equal(t, "environment_common_2", deployComponentProd[1].EnvironmentVariables["ENV_COMMON_2"])

	deployComponentDev := getRadixComponentsForEnv(ra, "dev", componentImages)
	assert.Equal(t, 2, len(deployComponentDev))

	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, 1, len(deployComponentDev[0].EnvironmentVariables))
	assert.Equal(t, "environment_common_1", deployComponentDev[0].EnvironmentVariables["ENV_COMMON_1"])

	assert.Equal(t, "comp_2", deployComponentDev[1].Name)
	assert.Equal(t, 1, len(deployComponentDev[1].EnvironmentVariables))
	assert.Equal(t, "environment_common_2", deployComponentDev[1].EnvironmentVariables["ENV_COMMON_2"])
}

func TestGetRadixComponentsForEnv_CommonResources(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}

	ra := utils.ARadixApplication().
		WithEnvironment("prod", "release").
		WithEnvironment("dev", "master").
		WithComponents(
			utils.NewApplicationComponentBuilder().
				WithName("comp_1").
				WithCommonResource(map[string]string{
					"memory": "64Mi",
					"cpu":    "250m",
				}, map[string]string{
					"memory": "128Mi",
					"cpu":    "500m",
				}).WithEnvironmentConfigs(
				utils.AnEnvironmentConfig().
					WithEnvironment("prod").
					WithResource(map[string]string{
						"memory": "128Mi",
						"cpu":    "500m",
					}, map[string]string{
						"memory": "256Mi",
						"cpu":    "750m",
					}))).BuildRA()

	deployComponentProd := getRadixComponentsForEnv(ra, "prod", componentImages)
	assert.Equal(t, 1, len(deployComponentProd))
	assert.Equal(t, "comp_1", deployComponentProd[0].Name)
	assert.Equal(t, "500m", deployComponentProd[0].Resources.Requests["cpu"])
	assert.Equal(t, "128Mi", deployComponentProd[0].Resources.Requests["memory"])
	assert.Equal(t, "750m", deployComponentProd[0].Resources.Limits["cpu"])
	assert.Equal(t, "256Mi", deployComponentProd[0].Resources.Limits["memory"])

	deployComponentDev := getRadixComponentsForEnv(ra, "dev", componentImages)
	assert.Equal(t, 1, len(deployComponentDev))
	assert.Equal(t, "comp_1", deployComponentDev[0].Name)
	assert.Equal(t, "250m", deployComponentDev[0].Resources.Requests["cpu"])
	assert.Equal(t, "64Mi", deployComponentDev[0].Resources.Requests["memory"])
	assert.Equal(t, "500m", deployComponentDev[0].Resources.Limits["cpu"])
	assert.Equal(t, "128Mi", deployComponentDev[0].Resources.Limits["memory"])
}

func Test_GetRadixComponents_NodeName(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: anyImage, ImagePath: anyImagePath}
	compGpu := "comp gpu"
	compGpuCount := "10"
	envGpu1 := "env1 gpu"
	envGpuCount1 := "20"
	envGpuCount2 := "30"
	envGpu3 := "env3 gpu"
	ra := utils.ARadixApplication().
		WithComponents(
			utils.AnApplicationComponent().
				WithName("comp").
				WithNode(v1.RadixNode{Gpu: compGpu, GpuCount: compGpuCount}).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("env1").
						WithNode(v1.RadixNode{Gpu: envGpu1, GpuCount: envGpuCount1}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env2").
						WithNode(v1.RadixNode{GpuCount: envGpuCount2}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env3").
						WithNode(v1.RadixNode{Gpu: envGpu3}),
					utils.AnEnvironmentConfig().
						WithEnvironment("env4"),
				),
		).BuildRA()

	t.Run("override job gpu and gpu-count with environment gpu and gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent := getRadixComponentsForEnv(ra, "env1", componentImages)
		assert.Equal(t, envGpu1, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent := getRadixComponentsForEnv(ra, "env2", componentImages)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, deployComponent[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		deployComponent := getRadixComponentsForEnv(ra, "env3", componentImages)
		assert.Equal(t, envGpu3, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		deployComponent := getRadixComponentsForEnv(ra, "env4", componentImages)
		assert.Equal(t, compGpu, deployComponent[0].Node.Gpu)
		assert.Equal(t, compGpuCount, deployComponent[0].Node.GpuCount)
	})
}
