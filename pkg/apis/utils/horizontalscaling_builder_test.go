package utils_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v2 "k8s.io/api/autoscaling/v2"
)

func Test_EmptyHorizontalScalingBuilder(t *testing.T) {
	set := utils.NewHorizontalScalingBuilder().Build()
	assert.Empty(t, set.Triggers)
	assert.Zero(t, set.MaxReplicas)
	assert.Nil(t, set.MinReplicas)
	assert.Nil(t, set.CooldownPeriod)
	assert.Nil(t, set.PollingInterval)
}

func Test_HorizontalScalingBuilder(t *testing.T) {
	set := utils.NewHorizontalScalingBuilder().
		WithMinReplicas(2).
		WithMaxReplicas(4).
		WithCooldownPeriod(10).
		WithPollingInterval(16).
		Build()

	require.NotNil(t, set.MinReplicas)
	assert.Equal(t, int32(2), *set.MinReplicas)

	require.NotNil(t, set.MaxReplicas)
	assert.Equal(t, int32(4), set.MaxReplicas)

	require.NotNil(t, set.CooldownPeriod)
	assert.Equal(t, int32(10), *set.CooldownPeriod)

	require.NotNil(t, set.PollingInterval)
	assert.Equal(t, int32(16), *set.PollingInterval)
}

func Test_HorizontalScalingBuilder_Triggers(t *testing.T) {
	var testScenarios = []struct {
		name     string
		actual   *v1.RadixHorizontalScaling
		expected v1.RadixHorizontalScalingTrigger
	}{
		{
			name:     "CPU",
			actual:   utils.NewHorizontalScalingBuilder().WithCPUTrigger(80).Build(),
			expected: v1.RadixHorizontalScalingTrigger{Name: "cpu", Cpu: &v1.RadixHorizontalScalingCPUTrigger{Value: 80, MetricType: v2.UtilizationMetricType}},
		},
		{
			name:     "Memory",
			actual:   utils.NewHorizontalScalingBuilder().WithMemoryTrigger(80).Build(),
			expected: v1.RadixHorizontalScalingTrigger{Name: "memory", Memory: &v1.RadixHorizontalScalingMemoryTrigger{Value: 80, MetricType: v2.UtilizationMetricType}},
		},
		{
			name:   "Cron",
			actual: utils.NewHorizontalScalingBuilder().WithCRONTrigger("10 * * * *", "20 * * * *", "Europe/Oslo", 30).Build(),
			expected: v1.RadixHorizontalScalingTrigger{Name: "cron", Cron: &v1.RadixHorizontalScalingCronTrigger{
				Start:           "10 * * * *",
				End:             "20 * * * *",
				Timezone:        "Europe/Oslo",
				DesiredReplicas: 30,
			}},
		},
		{
			name: "AzureServiceBus",
			actual: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", pointers.Ptr("queue-name"), pointers.Ptr("topic-name"), pointers.Ptr("subscription-name"), pointers.Ptr(5), pointers.Ptr(10)).
				Build(),
			expected: v1.RadixHorizontalScalingTrigger{Name: "azure-service-bus", AzureServiceBus: &v1.RadixHorizontalScalingAzureServiceBusTrigger{
				Namespace:              "anamespace",
				QueueName:              pointers.Ptr("queue-name"),
				TopicName:              pointers.Ptr("topic-name"),
				SubscriptionName:       pointers.Ptr("subscription-name"),
				MessageCount:           pointers.Ptr(5),
				ActivationMessageCount: pointers.Ptr(10),
				Authentication: v1.RadixHorizontalScalingAuthentication{
					Identity: v1.RequiredIdentity{Azure: v1.AzureIdentity{ClientId: "abcd"}},
				},
			}},
		},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			require.NotNil(t, testcase.actual)
			require.Len(t, testcase.actual.Triggers, 1)
			assert.Equal(t, testcase.expected, (testcase.actual.Triggers)[0])
		})
	}
}
