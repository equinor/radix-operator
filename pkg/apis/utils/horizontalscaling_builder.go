package utils

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v2 "k8s.io/api/autoscaling/v2"
)

type HorizontalScalingBuilderStruct struct {
	minReplicas     *int32
	maxReplicas     *int32
	cooldownPeriod  *int32
	triggers        []radixv1.RadixTrigger
	pollingInterval *int32
}

// NewHorizontalScalingBuilder Constructor for deployment actual
func NewHorizontalScalingBuilder() *HorizontalScalingBuilderStruct {
	return &HorizontalScalingBuilderStruct{}
}

func (h *HorizontalScalingBuilderStruct) Build() *radixv1.RadixHorizontalScaling {
	if h == nil {
		return nil
	}

	s := radixv1.RadixHorizontalScaling{
		Triggers: &h.triggers,
	}

	if h.minReplicas != nil {
		s.MinReplicas = h.minReplicas
	}
	if h.maxReplicas != nil {
		s.MaxReplicas = *h.maxReplicas
	}
	if h.cooldownPeriod != nil {
		s.CooldownPeriod = h.cooldownPeriod
	}
	if h.pollingInterval != nil {
		s.PollingInterval = h.pollingInterval
	}

	return &s
}

func (h *HorizontalScalingBuilderStruct) WithAzureServiceBusTrigger(namespace, clientId string, queueName, topicName, subscriptionName *string, messageCount, acitvationmessageCount *int) *HorizontalScalingBuilderStruct {
	authentication := radixv1.RadixHorizontalScalingAuthentication{
		Identity: radixv1.RequiredIdentity{
			Azure: radixv1.AzureIdentity{
				ClientId: clientId,
			},
		},
	}

	h.WithTrigger(radixv1.RadixTrigger{
		Name: "azure-service-bus",
		AzureServiceBus: &radixv1.RadixHorizontalScalingAzureServiceBusTrigger{
			Namespace:              namespace,
			QueueName:              queueName,
			TopicName:              topicName,
			SubscriptionName:       subscriptionName,
			MessageCount:           messageCount,
			ActivationMessageCount: acitvationmessageCount,
			Authentication:         authentication,
		},
	})

	return h
}

func (h *HorizontalScalingBuilderStruct) WithCooldownPeriod(cooldown int32) *HorizontalScalingBuilderStruct {
	h.cooldownPeriod = &cooldown
	return h
}

func (h *HorizontalScalingBuilderStruct) WithPollingInterval(pollingInterval int32) *HorizontalScalingBuilderStruct {
	h.pollingInterval = &pollingInterval
	return h
}

func (h *HorizontalScalingBuilderStruct) WithCPUTrigger(value int) *HorizontalScalingBuilderStruct {
	h.WithTrigger(radixv1.RadixTrigger{
		Name: "cpu",
		Cpu:  &radixv1.RadixHorizontalScalingCPUTrigger{Value: value, MetricType: v2.UtilizationMetricType},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithMemoryTrigger(value int) *HorizontalScalingBuilderStruct {
	h.WithTrigger(radixv1.RadixTrigger{
		Name:   "memory",
		Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: value, MetricType: v2.UtilizationMetricType},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithCRONTrigger(start, end, timezone string, desiredReplicas int) *HorizontalScalingBuilderStruct {
	h.WithTrigger(radixv1.RadixTrigger{
		Name: "cron",
		Cron: &radixv1.RadixHorizontalScalingCronTrigger{
			Start:           start,
			End:             end,
			Timezone:        timezone,
			DesiredReplicas: desiredReplicas,
		},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithTrigger(trigger radixv1.RadixTrigger) *HorizontalScalingBuilderStruct {
	h.triggers = append(h.triggers, trigger)
	return h
}

func (h *HorizontalScalingBuilderStruct) WithMaxReplicas(maxReplicas int32) *HorizontalScalingBuilderStruct {
	h.maxReplicas = &maxReplicas
	return h
}

func (h *HorizontalScalingBuilderStruct) WithMinReplicas(minReplicas int32) *HorizontalScalingBuilderStruct {
	h.minReplicas = &minReplicas
	return h
}
