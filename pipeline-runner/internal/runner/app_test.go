package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils/configcodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
)

func getValidConfig(t *testing.T) config.Config {
	t.Helper()

	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	configYaml, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "../../../pkg/apis/config/testdata/config-happypath.yaml"))
	require.NoError(t, err)

	var cfg config.Config
	require.NoError(t, configcodec.Decode(configYaml, &cfg))
	return cfg
}

func TestPrepareRun_NoRegistration_ReturnsError(t *testing.T) {
	radixclient := radix.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	pipelineDefinition, _ := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	cli := NewRunner(nil, radixclient, nil, nil, pipelineDefinition, "any-app")

	err := cli.PrepareRun(context.Background(), &model.PipelineArguments{})
	assert.ErrorContains(t, err, "failed to get RadixRegistration for app")
}

func TestPrepareRun_LoadsConfigFromConfigMap(t *testing.T) {
	const appName, configMapName, configMapNamespace = "any-app", "pipeline-config", "pipeline-namespace"
	expectedConfig := getValidConfig(t)
	expectedConfig.Common.ClusterName = "config-map-cluster"
	configYaml, err := configcodec.Encode(expectedConfig)
	require.NoError(t, err)
	require.NoError(t, configcodec.Decode(configYaml, &expectedConfig))

	radixClient := radix.NewSimpleClientset(&v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	dynamicClient := commonTest.CreateClient(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: configMapNamespace},
		Data:       map[string]string{"configYaml": string(configYaml)},
	})
	pipelineDefinition, err := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	require.NoError(t, err)
	cli := NewRunner(nil, radixClient, dynamicClient, nil, pipelineDefinition, appName)

	err = cli.PrepareRun(context.Background(), &model.PipelineArguments{
		ConfigMapName:      configMapName,
		ConfigMapNamespace: configMapNamespace,
	})

	require.NoError(t, err)
	require.NotNil(t, cli.pipelineInfo)
	assert.Equal(t, expectedConfig, cli.pipelineInfo.Cfg)
}

func TestPrepareRun_ConfigMapErrors(t *testing.T) {
	const appName, configMapName, configMapNamespace = "any-app", "pipeline-config", "pipeline-namespace"
	tests := map[string]struct {
		configMap     *corev1.ConfigMap
		expectedError string
	}{
		"config map does not exist": {
			expectedError: "failed to read configmap pipeline-namespace/pipeline-config",
		},
		"config map does not contain configYaml": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: configMapNamespace},
			},
			expectedError: "does not contain key 'configYaml'",
		},
		"config is invalid": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: configMapNamespace},
				Data:       map[string]string{"configYaml": "{"},
			},
			expectedError: "failed to decode config data",
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			radixClient := radix.NewSimpleClientset(&v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
			dynamicClient := commonTest.CreateClient()
			if testCase.configMap != nil {
				dynamicClient = commonTest.CreateClient(testCase.configMap)
			}
			pipelineDefinition, err := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
			require.NoError(t, err)
			cli := NewRunner(nil, radixClient, dynamicClient, nil, pipelineDefinition, appName)

			err = cli.PrepareRun(context.Background(), &model.PipelineArguments{
				ConfigMapName:      configMapName,
				ConfigMapNamespace: configMapNamespace,
			})

			assert.ErrorContains(t, err, testCase.expectedError)
		})
	}
}

func TestPrepareRun_ConfigOverrideFileTakesPrecedence(t *testing.T) {
	const appName = "any-app"
	expectedConfig := getValidConfig(t)
	expectedConfig.Common.ClusterName = "override-file-cluster"
	configYaml, err := configcodec.Encode(expectedConfig)
	require.NoError(t, err)
	require.NoError(t, configcodec.Decode(configYaml, &expectedConfig))
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, configYaml, 0o600))
	t.Setenv("CONFIG_OVERRIDE_FILENAME", configFile)

	radixClient := radix.NewSimpleClientset(&v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	pipelineDefinition, err := pipeline.GetPipelineFromName(string(v1.BuildDeploy))
	require.NoError(t, err)
	cli := NewRunner(nil, radixClient, commonTest.CreateClient(), nil, pipelineDefinition, appName)

	err = cli.PrepareRun(context.Background(), &model.PipelineArguments{
		ConfigMapName:      "missing-config-map",
		ConfigMapNamespace: "missing-namespace",
	})

	require.NoError(t, err)
	require.NotNil(t, cli.pipelineInfo)
	assert.Equal(t, expectedConfig, cli.pipelineInfo.Cfg)
}
