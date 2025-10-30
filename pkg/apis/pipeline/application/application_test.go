package application_test

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ParseRadixApplication_MismatchAppName(t *testing.T) {
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
	_, err = application.ParseRadixApplication(context.Background(), radixClient, registeredAppName, configFileContent)
	require.EqualError(t, err, "the application name testapp in the radixconfig file does not match the registered application name mismatching-app-name")
}

func Test_ParseRadixApplication_MatchAppName(t *testing.T) {
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
	ra, err := application.ParseRadixApplication(context.Background(), radixClient, radixconfigAppName, configFileContent)
	require.NoError(t, err)
	require.Equal(t, radixconfigAppName, ra.GetName(), "Application name should be the same as in the radixconfig file")
}
