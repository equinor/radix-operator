package utils

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type HorizontalScalingBuilder interface {
	WithMinReplicas(minReplicas int32) HorizontalScalingBuilder
	WithMaxReplicas(maxReplicas int32) HorizontalScalingBuilder
	WithCooldownPeriod(cooldown int32) HorizontalScalingBuilder
	WithPollingInterval(pollingInterval int32) HorizontalScalingBuilder

	WithCPUTrigger(value int) HorizontalScalingBuilder
	WithMemoryTrigger(value int) HorizontalScalingBuilder
	WithCRONTrigger(start, stop, timezone string, desiredReplicas int) HorizontalScalingBuilder
	WithAzureServiceBusTrigger(queueName, topicName, subscriptionName, clientId *string, messageCount, acitvationmessageCount *int) HorizontalScalingBuilder
	WithTrigger(trigger radixv1.RadixTrigger) HorizontalScalingBuilder

	Build() *radixv1.RadixHorizontalScaling
}
type HorizontalScalingBuilderStruct struct {
	minReplicas     *int32
	maxReplicas     *int32
	cooldownPeriod  *int32
	triggers        []radixv1.RadixTrigger
	pollingInterval *int32
}

func (h *HorizontalScalingBuilderStruct) WithAzureServiceBusTrigger(queueName, topicName, subscriptionName, clientId *string, messageCount, acitvationmessageCount *int) HorizontalScalingBuilder {
	authentication := radixv1.RadixHorizontalScalingAuthentication{
		Identity: radixv1.Identity{
			Azure: nil,
		},
	}

	if clientId != nil {
		authentication.Identity.Azure = &radixv1.AzureIdentity{
			ClientId: *clientId,
		}
	}

	h.WithTrigger(radixv1.RadixTrigger{
		Name: "azure service bus",
		AzureServiceBus: &radixv1.RadixHorizontalScalingAzureServiceBusTrigger{
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

func (h *HorizontalScalingBuilderStruct) WithCooldownPeriod(cooldown int32) HorizontalScalingBuilder {
	h.cooldownPeriod = &cooldown
	return h
}

func (h *HorizontalScalingBuilderStruct) WithPollingInterval(pollingInterval int32) HorizontalScalingBuilder {
	h.pollingInterval = &pollingInterval
	return h
}

func (h *HorizontalScalingBuilderStruct) WithCPUTrigger(value int) HorizontalScalingBuilder {
	h.WithTrigger(radixv1.RadixTrigger{
		Name: "cpu",
		Cpu:  &radixv1.RadixHorizontalScalingCPUTrigger{Value: value},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithMemoryTrigger(value int) HorizontalScalingBuilder {
	h.WithTrigger(radixv1.RadixTrigger{
		Name:   "memory",
		Memory: &radixv1.RadixHorizontalScalingMemoryTrigger{Value: value},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithCRONTrigger(start, stop, timezone string, desiredReplicas int) HorizontalScalingBuilder {
	h.WithTrigger(radixv1.RadixTrigger{
		Name: "cron",
		Cron: &radixv1.RadixHorizontalScalingCronTrigger{
			Start:           start,
			Stop:            stop,
			Timezone:        timezone,
			DesiredReplicas: desiredReplicas,
		},
	})
	return h
}

func (h *HorizontalScalingBuilderStruct) WithTrigger(trigger radixv1.RadixTrigger) HorizontalScalingBuilder {
	h.triggers = append(h.triggers, trigger)
	return h
}

func (h *HorizontalScalingBuilderStruct) Build() *radixv1.RadixHorizontalScaling {
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

func (h *HorizontalScalingBuilderStruct) WithMaxReplicas(maxReplicas int32) HorizontalScalingBuilder {
	h.maxReplicas = &maxReplicas
	return h
}

func (h *HorizontalScalingBuilderStruct) WithMinReplicas(minReplicas int32) HorizontalScalingBuilder {
	h.minReplicas = &minReplicas
	return h
}

// NewHorizontalScalingBuilder Constructor for deployment actual
func NewHorizontalScalingBuilder() HorizontalScalingBuilder {
	return &HorizontalScalingBuilderStruct{}
}
