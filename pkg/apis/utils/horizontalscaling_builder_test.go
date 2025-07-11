package utils_test

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		actual   *radixv1.RadixHorizontalScaling
		expected radixv1.RadixHorizontalScalingTrigger
	}{
		{
			name:     "CPU",
			actual:   utils.NewHorizontalScalingBuilder().WithCPUTrigger(80).Build(),
			expected: radixv1.RadixHorizontalScalingTrigger{Name: "cpu", Cpu: &radixv1.RadixHorizontalScalingCPUTrigger{Value: 80}},
		},
		{
			name:     "Memory",
			actual:   utils.NewHorizontalScalingBuilder().WithMemoryTrigger(80).Build(),
			expected: radixv1.RadixHorizontalScalingTrigger{Name: "memory", Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: 80}},
		},
		{
			name:   "Cron",
			actual: utils.NewHorizontalScalingBuilder().WithCRONTrigger("10 * * * *", "20 * * * *", "Europe/Oslo", 30).Build(),
			expected: radixv1.RadixHorizontalScalingTrigger{Name: "cron", Cron: &radixv1.RadixHorizontalScalingCronTrigger{
				Start:           "10 * * * *",
				End:             "20 * * * *",
				Timezone:        "Europe/Oslo",
				DesiredReplicas: 30,
			}},
		},
		{
			name: "AzureServiceBus",
			actual: utils.NewHorizontalScalingBuilder().WithAzureServiceBusTrigger("anamespace", "abcd", "queue-name", "topic-name", "subscription-name", "", pointers.Ptr(5), pointers.Ptr(10)).
				Build(),
			expected: radixv1.RadixHorizontalScalingTrigger{Name: "azure-service-bus", AzureServiceBus: &radixv1.RadixHorizontalScalingAzureServiceBusTrigger{
				Namespace:              "anamespace",
				QueueName:              "queue-name",
				TopicName:              "topic-name",
				SubscriptionName:       "subscription-name",
				MessageCount:           pointers.Ptr(5),
				ActivationMessageCount: pointers.Ptr(10),
				Authentication: radixv1.RadixHorizontalScalingAuthentication{
					Identity: radixv1.RadixHorizontalScalingRequiredIdentity{Azure: radixv1.AzureIdentity{ClientId: "abcd"}},
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
