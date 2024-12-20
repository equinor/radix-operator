package deployment

import (
	"context"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/stretchr/testify/assert"
)

func Test_CreateOrUpdateCsiAzureKeyVaultResources(t *testing.T) {
	appName := "app"
	namespace := "some-namespace"
	environment := "some-env"
	componentName1, componentNameLong := "component1", "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit"
	type expectedVolumeProps struct {
		expectedVolumeNamePrefix         string
		expectedVolumeMountPath          string
		expectedNodePublishSecretRefName string
		expectedVolumeAttributePrefixes  map[string]string
	}
	scenarios := []struct {
		name                    string
		deployComponentBuilders []utils.DeployCommonComponentBuilder
		componentName           string
		azureKeyVaults          []v1.RadixAzureKeyVault
		expectedVolumeProps     []expectedVolumeProps
		radixVolumeMounts       []v1.RadixVolumeMount
	}{
		{
			name:                "No Azure Key volumes as no RadixAzureKeyVault-s",
			componentName:       componentName1,
			azureKeyVaults:      []v1.RadixAzureKeyVault{},
			expectedVolumeProps: []expectedVolumeProps{},
		},
		{
			name:           "No Azure Key volumes as no secret names in secret object",
			componentName:  componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{{Name: "kv1"}},
		},
		{
			name:          "One Azure Key volume for one secret objects secret name",
			componentName: componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:  "kv1",
				Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
			}},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv1-",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv1",
					expectedNodePublishSecretRefName: "component1-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv1-",
					},
				},
			},
		},
		{
			name:          "Multiple Azure Key volumes for each RadixAzureKeyVault",
			componentName: componentName1,
			azureKeyVaults: []v1.RadixAzureKeyVault{
				{
					Name:  "kv1",
					Path:  utils.StringPtr("/mnt/customPath"),
					Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
				},
				{
					Name:  "kv2",
					Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2", EnvVar: "SECRET_REF2"}},
				},
			},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv1-",
					expectedVolumeMountPath:          "/mnt/customPath",
					expectedNodePublishSecretRefName: "component1-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv1-",
					},
				},
				{
					expectedVolumeNamePrefix:         "component1-az-keyvault-opaque-kv2-",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv2",
					expectedNodePublishSecretRefName: "component1-kv2-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "component1-az-keyvault-kv2-",
					},
				},
			},
		},
		{
			name:          "Volume name should be trimmed when exceeding 63 chars",
			componentName: componentNameLong,
			azureKeyVaults: []v1.RadixAzureKeyVault{{
				Name:  "kv1",
				Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1", EnvVar: "SECRET_REF1"}},
			}},
			expectedVolumeProps: []expectedVolumeProps{
				{
					expectedVolumeNamePrefix:         "a-very-long-component-name-that-exceeds-63-kubernetes-vol",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv1",
					expectedNodePublishSecretRefName: "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "a-very-long-component-name-that-exceeds-63-kubernetes-volume-name-limit-az-keyvault-kv1-",
					},
				},
			},
		},
	}
	t.Run("CSI Azure Key vault volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			deployment := getDeployment(t)
			radixDeployment := buildRdWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			radixDeployComponent := radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.CreateAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumes, err := volumemount.GetVolumes(context.Background(), deployment.kubeutil, namespace, radixDeployComponent, radixDeployment.GetName(), nil)
			assert.Nil(t, err)
			assert.Len(t, volumes, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumes); i++ {
				volume := volumes[i]
				assert.Less(t, len(volume.Name), 64, "volume name is too long")
				assert.NotNil(t, volume.CSI)
				assert.NotNil(t, volume.CSI.VolumeAttributes)
				assert.NotNil(t, volume.CSI.NodePublishSecretRef)
				assert.Equal(t, "secrets-store.csi.k8s.io", volume.CSI.Driver)

				volumeProp := scenario.expectedVolumeProps[i]
				for attrKey, attrValue := range volumeProp.expectedVolumeAttributePrefixes {
					spcValue, exists := volume.CSI.VolumeAttributes[attrKey]
					assert.True(t, exists)
					assert.True(t, strings.HasPrefix(spcValue, attrValue))
				}
				assert.True(t, strings.Contains(volume.Name, volumeProp.expectedVolumeNamePrefix))
				assert.Equal(t, volumeProp.expectedNodePublishSecretRefName, volume.CSI.NodePublishSecretRef.Name)
			}
		}
	})

	t.Run("CSI Azure Key vault volume mounts", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			deployment := getDeployment(t)
			radixDeployment := buildRdWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			radixDeployComponent := radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.CreateAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumeMounts, err := volumemount.GetRadixDeployComponentVolumeMounts(radixDeployComponent, radixDeployment.GetName())
			assert.Nil(t, err)
			assert.Len(t, volumeMounts, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumeMounts); i++ {
				volumeMount := volumeMounts[i]
				volumeProp := scenario.expectedVolumeProps[i]
				assert.Less(t, len(volumeMount.Name), 64, "volumemount name is too long")
				assert.True(t, strings.Contains(volumeMount.Name, volumeProp.expectedVolumeNamePrefix))
				assert.Equal(t, volumeProp.expectedVolumeMountPath, volumeMount.MountPath)
				assert.True(t, volumeMount.ReadOnly)
			}
		}
	})
}

func getDeployment(t *testing.T) *Deployment {
	tu, client, kubeUtil, radixClient, kedaClient, prometheusClient, _, certClient := SetupTest(t)
	rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixClient, kedaClient, prometheusClient, certClient,
		utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().
				WithName("comp1")).
			WithAppName("any-app").
			WithEnvironment("test"))
	return &Deployment{radixclient: radixClient, kubeutil: kubeUtil, radixDeployment: rd, config: &testConfig}
}

func buildRdWithComponentBuilders(appName string, environment string, componentBuilders func() []utils.DeployComponentBuilder) *v1.RadixDeployment {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(componentBuilders()...).
		BuildRD()
}
