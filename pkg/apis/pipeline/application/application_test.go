package application_test

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_CreateRadixApplication_LimitMemoryIsTakenFromRequestsMemory(t *testing.T) {
	const (
		sampleApp = "./testdata/radixconfig.yaml"
	)

	radixClient := radixfake.NewSimpleClientset()
	appName := "testapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	ra, err := application.CreateRadixApplication(context.Background(), radixClient, appName, &dnsalias.DNSConfig{}, string(configFileContent))
	require.NoError(t, err)
	assert.Equal(t, "100Mi", ra.Spec.Components[0].Resources.Requests["memory"], "server1 invalid resource requests memory")
	assert.Equal(t, "100Mi", ra.Spec.Components[0].Resources.Limits["memory"], "server1 invalid resource limits memory")
	assert.Equal(t, "100Mi", ra.Spec.Components[1].Resources.Requests["memory"], "server2 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[1].Resources.Limits["memory"], "server2 invalid resource limits memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[2].Resources.Requests["memory"], "server3 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[2].Resources.Limits["memory"], "server3 invalid resource limits memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[3].Resources.Requests["memory"], "server4 invalid resource requests memory")
	assert.Equal(t, "200Mi", ra.Spec.Components[3].Resources.Limits["memory"], "server4 invalid resource limits memory")

	err = validate.CanRadixApplicationBeInserted(context.Background(), radixClient, ra, nil)
	assert.NoError(t, err)
}

func Test_CreateRadixApplication_MismatchAppName(t *testing.T) {
	const (
		sampleApp = "./testdata/radixconfig.yaml"
	)

	radixClient := radixfake.NewSimpleClientset()
	const (
		registeredAppName  = "mismatching-app-name"
		radixconfigAppName = "testapp"
	)
	rr := utils.NewRegistrationBuilder().WithName(registeredAppName).BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	_, err = application.CreateRadixApplication(context.Background(), radixClient, registeredAppName, &dnsalias.DNSConfig{}, string(configFileContent))
	require.EqualError(t, err, "the application name testapp in the radixconfig file does not match the registered application name mismatching-app-name")
}

func Test_CreateRadixApplication_MatchAppName(t *testing.T) {
	const (
		sampleApp = "./testdata/radixconfig.yaml"
	)

	radixClient := radixfake.NewSimpleClientset()
	const (
		radixconfigAppName = "testapp"
	)
	rr := utils.NewRegistrationBuilder().WithName(radixconfigAppName).BuildRR()
	_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	configFileContent, err := os.ReadFile(sampleApp)
	require.NoError(t, err)
	ra, err := application.CreateRadixApplication(context.Background(), radixClient, radixconfigAppName, &dnsalias.DNSConfig{}, string(configFileContent))
	require.NoError(t, err)
	require.Equal(t, radixconfigAppName, ra.GetName(), "Application name should be the same as in the radixconfig file")
}
