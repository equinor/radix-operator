package deployment

import (
	"context"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CreateOrUpdateCsiAzureKeyVaultResources(t *testing.T) {
	const (
		appName           = "app"
		namespace         = "some-namespace"
		environment       = "some-env"
		componentName1    = "component1"
		componentNameLong = "max-long-component-name-0123456789012345678901234"
	)
	type expectedVolumeProps struct {
		expectedVolumeNamePrefix         string
		expectedVolumeMountPath          string
		expectedNodePublishSecretRefName string
		expectedVolumeAttributePrefixes  map[string]string
	}
	scenarios := []struct {
		name                string
		componentName       string
		azureKeyVaults      []v1.RadixAzureKeyVault
		expectedVolumeProps []expectedVolumeProps
		radixVolumeMounts   []v1.RadixVolumeMount
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
					expectedVolumeNamePrefix:         "max-long-component-name-0123456789012345678901234-az-keyv",
					expectedVolumeMountPath:          "/mnt/azure-key-vault/kv1",
					expectedNodePublishSecretRefName: "max-long-component-name-0123456789012345678901234-kv1-csiazkvcreds",
					expectedVolumeAttributePrefixes: map[string]string{
						"secretProviderClass": "max-long-component-name-0123456789012345678901234-az-keyvault-kv1-",
					},
				},
			},
		},
	}
	t.Run("CSI Azure Key vault volumes", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			t.Logf("Test case %s", scenario.name)
			rdBuilder := getRdBuilderWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			deployment := getDeployment(t, rdBuilder)
			radixDeployComponent := deployment.radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.CreateAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}
			volumes, err := volumemount.GetVolumes(context.Background(), deployment.kubeutil, namespace, radixDeployComponent, deployment.radixDeployment.GetName(), nil)
			require.NoError(t, err, "failed to get volumes")
			assert.Len(t, volumes, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumes); i++ {
				volume := volumes[i]
				assert.Less(t, len(volume.Name), 64, "volume name is too long")
				assert.NotNil(t, volume.CSI, "CSI should ne not nil")
				assert.NotEmpty(t, volume.CSI.VolumeAttributes[volumemount.CsiVolumeSourceVolumeAttributeSecretProviderClass], "VolumeAttributes should not be empty")
				assert.NotNil(t, volume.CSI.NodePublishSecretRef, "NodePublishSecretRef should not be nil")
				assert.Equal(t, volumemount.CsiVolumeSourceDriverSecretStore, volume.CSI.Driver, "Volume driver should be %s, but it is %s", volumemount.CsiVolumeSourceDriverSecretStore, volume.CSI.Driver)

				volumeProp := scenario.expectedVolumeProps[i]
				for attrKey, attrValue := range volumeProp.expectedVolumeAttributePrefixes {
					spcValue, exists := volume.CSI.VolumeAttributes[attrKey]
					assert.True(t, exists)
					assert.True(t, strings.HasPrefix(spcValue, attrValue), "SecretProviderClass name should have a prefix %s, but it has name %s", attrValue, spcValue)
				}
				assert.True(t, strings.HasPrefix(volume.Name, volumeProp.expectedVolumeNamePrefix), "Volume name should have prefix %s, but it is %s", volumeProp.expectedVolumeNamePrefix, volume.Name)
				assert.Equal(t, volumeProp.expectedNodePublishSecretRefName, volume.CSI.NodePublishSecretRef.Name, "NodePublishSecretRef name should be %s, but it is %s", volumeProp.expectedNodePublishSecretRefName, volume.CSI.NodePublishSecretRef.Name)
			}
		}
	})

	t.Run("CSI Azure Key vault volume mounts", func(t *testing.T) {
		//t.Parallel()
		for _, scenario := range scenarios {
			rdBuilder := getRdBuilderWithComponentBuilders(appName, environment, func() []utils.DeployComponentBuilder {
				var builders []utils.DeployComponentBuilder
				builders = append(builders, utils.NewDeployComponentBuilder().
					WithName(scenario.componentName).
					WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: scenario.azureKeyVaults}))
				return builders
			})
			deployment := getDeployment(t, rdBuilder)
			radixDeployComponent := deployment.radixDeployment.GetComponentByName(scenario.componentName)
			for _, azureKeyVault := range scenario.azureKeyVaults {
				spc, err := deployment.CreateAzureKeyVaultSecretProviderClassForRadixDeployment(context.Background(), namespace, appName, radixDeployComponent.GetName(), azureKeyVault)
				if err != nil {
					t.Log(err.Error())
				} else {
					t.Logf("created secret provider class %s", spc.Name)
				}
			}

			volumeMounts, err := volumemount.GetRadixDeployComponentVolumeMounts(radixDeployComponent, deployment.radixDeployment.GetName())
			require.Nil(t, err)

			assert.Len(t, volumeMounts, len(scenario.expectedVolumeProps))
			if len(scenario.expectedVolumeProps) == 0 {
				continue
			}

			for i := 0; i < len(volumeMounts); i++ {
				volumeMount := volumeMounts[i]
				volumeProp := scenario.expectedVolumeProps[i]
				assert.Less(t, len(volumeMount.Name), 64, "volumemount name is too long")
				assert.True(t, strings.HasPrefix(volumeMount.Name, volumeProp.expectedVolumeNamePrefix),
					"VolumeMount name should have prefix %s, but it is  %s", volumeProp.expectedVolumeNamePrefix, volumeMount.Name)
				assert.Equal(t, volumeProp.expectedVolumeMountPath, volumeMount.MountPath, "VolumeMount mount path should be %s, but it is %s", volumeProp.expectedVolumeMountPath, volumeMount.MountPath)
				assert.True(t, volumeMount.ReadOnly, "VolumeMount should be read only")
			}
		}
	})
}

func getDeployment(t *testing.T, deploymentBuilder utils.DeploymentBuilder) *Deployment {
	tu, client, kubeUtil, radixClient, kedaClient, prometheusClient, _, certClient := SetupTest(t)
	rd, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixClient, kedaClient, prometheusClient, certClient,
		deploymentBuilder)
	require.NoError(t, err)
	return &Deployment{radixclient: radixClient, kubeutil: kubeUtil, radixDeployment: rd, config: &testConfig}
}

func getRdBuilderWithComponentBuilders(appName string, environment string, componentBuilders func() []utils.DeployComponentBuilder) utils.DeploymentBuilder {
	return utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(componentBuilders()...)
}
