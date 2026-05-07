package metrics_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/api/metrics/mock"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	appName1 = "anyapp"
)

func Test_handler_GetReplicaResourcesUtilization(t *testing.T) {
	radixclient := radixfake.NewSimpleClientset() //nolint:staticcheck

	ra := utils.ARadixApplication().BuildRA()
	_, err := radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(appName1)).Create(context.Background(), ra, v1.CreateOptions{})
	require.NoError(t, err)

	scenarios := []struct {
		name    string
		appName string
		envName string
	}{
		{
			name:    "Get utilization in all environments",
			appName: appName1,
		},
		{
			name:    "Get utilization in specific environments",
			appName: appName1,
			envName: "test",
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)

			cpuReqs := []metrics.LabeledResults{
				{Value: 1, Environment: "test", Component: "app", Pod: "app-abcd-1"},
				{Value: 2, Environment: "test", Component: "app", Pod: "app-abcd-2"},
			}
			cpuAvg := []metrics.LabeledResults{
				{Value: 0.5, Environment: "test", Component: "app", Pod: "app-abcd-1"},
				{Value: 0.7, Environment: "test", Component: "app", Pod: "app-abcd-2"},
			}
			memReqs := []metrics.LabeledResults{
				{Value: 100, Environment: "test", Component: "app", Pod: "app-abcd-1"},
				{Value: 200, Environment: "test", Component: "app", Pod: "app-abcd-2"},
			}
			MemMax := []metrics.LabeledResults{
				{Value: 50, Environment: "test", Component: "app", Pod: "app-abcd-1"},
				{Value: 100, Environment: "test", Component: "app", Pod: "app-abcd-2"},
			}

			client.EXPECT().GetCpuRequests(gomock.Any(), ts.appName, ts.envName, []string{"app"}).Times(1).Return(cpuReqs, nil)
			client.EXPECT().GetCpuAverage(gomock.Any(), ts.appName, ts.envName, []string{"app"}, "24h").Times(1).Return(cpuAvg, nil)
			client.EXPECT().GetMemoryRequests(gomock.Any(), ts.appName, ts.envName, []string{"app"}).Times(1).Return(memReqs, nil)
			client.EXPECT().GetMemoryMaximum(gomock.Any(), ts.appName, ts.envName, []string{"app"}, "24h").Times(1).Return(MemMax, nil)

			metricsHandler := metrics.NewHandler(client)
			response, err := metricsHandler.GetReplicaResourcesUtilization(context.Background(), radixclient, ts.appName, ts.envName)
			assert.NoError(t, err)

			require.NotNil(t, response)
			require.Contains(t, response.Environments, "test")
			require.Contains(t, response.Environments["test"].Components, "app")
			assert.Contains(t, response.Environments["test"].Components["app"].Replicas, "app-abcd-1")
			assert.Contains(t, response.Environments["test"].Components["app"].Replicas, "app-abcd-2")

			assert.EqualValues(t, 1, response.Environments["test"].Components["app"].Replicas["app-abcd-1"].CpuRequests)
			assert.EqualValues(t, 0.5, response.Environments["test"].Components["app"].Replicas["app-abcd-1"].CpuAverage)
			assert.EqualValues(t, 100, response.Environments["test"].Components["app"].Replicas["app-abcd-1"].MemoryRequests)
			assert.EqualValues(t, 50, response.Environments["test"].Components["app"].Replicas["app-abcd-1"].MemoryMaximum)

			assert.EqualValues(t, 2, response.Environments["test"].Components["app"].Replicas["app-abcd-2"].CpuRequests)
			assert.EqualValues(t, 0.7, response.Environments["test"].Components["app"].Replicas["app-abcd-2"].CpuAverage)
			assert.EqualValues(t, 200, response.Environments["test"].Components["app"].Replicas["app-abcd-2"].MemoryRequests)
			assert.EqualValues(t, 100, response.Environments["test"].Components["app"].Replicas["app-abcd-2"].MemoryMaximum)

			assert.NotEmpty(t, response)
		})
	}
}
