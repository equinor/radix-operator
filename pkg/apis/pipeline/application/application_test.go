package application_test

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseRadixApplication_LimitMemoryIsTakenFromRequestsMemory(t *testing.T) {
	const (
		sampleApp = "./testdata/radixconfig.yaml"
		appName   = "testapp"
	)

	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	ra, err := application.ParseRadixApplication(appName, configFileContent)
	require.NoError(t, err)
	assert.Equal(t, "100Mi", ra.Spec.Components[0].Resources.Requests["memory"], "server1 invalid resource requests memory")
	assert.Equal(t, "100Mi", ra.Spec.Components[0].Resources.Limits["memory"], "server1 invalid resource limits memory")
	assert.Equal(t, "100Mi", ra.Spec.Components[1].Resources.Requests["memory"], "server2 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[1].Resources.Limits["memory"], "server2 invalid resource limits memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[2].Resources.Requests["memory"], "server3 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[2].Resources.Limits["memory"], "server3 invalid resource limits memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[3].Resources.Requests["memory"], "server4 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[3].Resources.Limits["memory"], "server4 invalid resource limits memory")
}

func Test_ParseRadixApplication_MismatchAppName(t *testing.T) {
	const (
		sampleApp          = "./testdata/radixconfig.yaml"
		registeredAppName  = "mismatching-app-name"
		radixconfigAppName = "testapp"
	)

	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	_, err = application.ParseRadixApplication(registeredAppName, configFileContent)
	require.EqualError(t, err, "the application name testapp in the radixconfig file does not match the registered application name mismatching-app-name")
}

func Test_ParseRadixApplication_MatchAppName(t *testing.T) {
	const (
		sampleApp          = "./testdata/radixconfig.yaml"
		radixconfigAppName = "testapp"
	)

	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	ra, err := application.ParseRadixApplication(radixconfigAppName, configFileContent)
	require.NoError(t, err)
	require.Equal(t, radixconfigAppName, ra.GetName(), "Application name should be the same as in the radixconfig file")
}
