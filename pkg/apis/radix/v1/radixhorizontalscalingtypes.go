package v1

import (
	"github.com/equinor/radix-common/utils/pointers"
	// "github.com/equinor/radix-operator/pkg/apis/deployment"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

const (
	DefaultTargetCPUUtilizationPercentage = 80
)

// RadixHorizontalScaling defines configuration for horizontal pod autoscaler.
type RadixHorizontalScaling struct {
	// Defines the minimum number of replicas.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Defines the maximum number of replicas.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// PollingInterval configures how often to check each trigger on. Defaults to 30sec
	PollingInterval *int32 `json:"pollingInterval"` // 30
	// CooldownPeriod to wait after the last trigger reported active before scaling the resource back to 0. Defaults to 5min
	CooldownPeriod *int32 `json:"cooldownPeriod"` // 300

	// Defines the resource usage parameters for the horizontal pod autoscaler.
	// +optional
	RadixHorizontalScalingResources *RadixHorizontalScalingResources `json:"resources,omitempty"` // Deprecated: Use CPU and/or Memory triggers instead

	// Defines a list of triggers the component replicas will scale on.
	// +optional
	Triggers *[]RadixTrigger `json:"triggers,omitempty"`
}

type RadixHorizontalScalingResource struct {
	// Defines the resource usage which triggers scaling for the horizontal pod autoscaler.
	// +kubebuilder:validation:Minimum=1
	AverageUtilization *int32 `json:"averageUtilization"`
}

type RadixHorizontalScalingResources struct {
	// Defines the CPU usage parameters for the horizontal pod autoscaler.
	// +optional
	Cpu *RadixHorizontalScalingResource `json:"cpu,omitempty"`

	// Defines the memory usage parameters for the horizontal pod autoscaler.
	// +optional
	Memory *RadixHorizontalScalingResource `json:"memory,omitempty"`
}

// RadixTrigger defines configuration for a specific trigger.
type RadixTrigger struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Name            string                                        `json:"name"`
	Cpu             *RadixHorizontalScalingCPUTrigger             `json:"cpu,omitempty"`
	Memory          *RadixHorizontalScalingMemoryTrigger          `json:"memory,omitempty"`
	Cron            *RadixHorizontalScalingCronTrigger            `json:"cron,omitempty"`
	AzureServiceBus *RadixHorizontalScalingAzureServiceBusTrigger `json:"azureServiceBus,omitempty"`
}

type RadixHorizontalScalingCPUTrigger struct {
	// Defines the type of metric to use. Options are Utilization or AverageValue. Defaults to Utilization.
	// +optional
	MetricType autoscalingv2.MetricTargetType `json:"metricType,omitempty"`

	// Value to trigger scaling actions for:
	// When using Utilization, the target value is the average of the resource metric across all relevant pods, represented as a percentage of the requested value of the resource for the pods.
	// When using AverageValue, the target value is the target value of the average of the metric across all relevant pods (quantity).
	Value int `json:"value"`
}
type RadixHorizontalScalingMemoryTrigger struct {
	// Defines the type of metric to use. Options are Utilization or AverageValue. Defaults to Utilization.
	// +optional
	MetricType autoscalingv2.MetricTargetType `json:"metricType,omitempty"` // TODO: Copy values and only allow Utilization and AverageValue

	// Value to trigger scaling actions for:
	// When using Utilization, the target value is the average of the resource metric across all relevant pods, represented as a percentage of the requested value of the resource for the pods.
	// When using AverageValue, the target value is the target value of the average of the metric across all relevant pods (quantity).
	Value int `json:"value"`
}
type RadixHorizontalScalingCronTrigger struct {
	// Start is a Cron expression indicating the start of the cron schedule.
	Start string `json:"start"`

	// End is a Cron expression indicating the End of the cron schedule.
	End string `json:"stop"`

	// Timezone One of the acceptable values from the IANA Time Zone Database. The list of timezones can be found at https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
	Timezone string `json:"timezone"`

	// DesiredReplicas Number of replicas to which the resource has to be scaled between the start and end of the cron schedule.
	DesiredReplicas int `json:"desiredReplicas"`
}

type RadixHorizontalScalingAzureServiceBusTrigger struct {
	// Namespace - Name of the Azure Service Bus namespace that contains your queue or topic.
	Namespace string `json:"namespace"`

	// QueueName selects the target queue. QueueName wil take precedence over TopicName.
	QueueName *string `json:"queueName,omitempty"`

	// TopicName selectes the target topic, alsoe requires SubscriptionName to be set.
	TopicName *string `json:"topicName,omitempty"`
	// SubscriptionName is required when TopicName is set.
	SubscriptionName *string `json:"subscriptionName,omitempty"`

	// MessageCount  - Amount of active messages in your Azure Service Bus queue or topic to scale on. Defaults to 5 messages
	MessageCount *int `json:"messageCount,omitempty"`
	// ActivationMessageCount = Target value for activating the scaler. (Default: 0, Optional)
	ActivationMessageCount *int `json:"activationMessageCount,omitempty"`

	// Azure Service Bus requires Workload Identity configured with a ClientID
	Authentication RadixHorizontalScalingAuthentication `json:"authentication"`
}
type RadixHorizontalScalingAuthentication struct {
	Identity RequiredIdentity `json:"identity"`
}

// NormalizeConfig copies, migrate deprecations and add defaults to configuration
func (c *RadixHorizontalScaling) NormalizeConfig() *RadixHorizontalScaling {
	// This method could probably return a v2alpha1.RadixHorizontalScaling when infrastructure for that is ready
	if c == nil {
		return nil
	}

	config := c.DeepCopy()
	config.RadixHorizontalScalingResources = nil

	if config.Triggers == nil && c.RadixHorizontalScalingResources != nil {
		config.Triggers = &[]RadixTrigger{}

		if c.RadixHorizontalScalingResources.Cpu != nil && c.RadixHorizontalScalingResources.Cpu.AverageUtilization != nil {
			*config.Triggers = append(*config.Triggers, RadixTrigger{
				Name: "CPU",
				Cpu: &RadixHorizontalScalingCPUTrigger{
					MetricType: autoscalingv2.UtilizationMetricType,
					Value:      int(*c.RadixHorizontalScalingResources.Cpu.AverageUtilization),
				},
			})
		}

		if c.RadixHorizontalScalingResources.Memory != nil && c.RadixHorizontalScalingResources.Memory.AverageUtilization != nil {
			*config.Triggers = append(*config.Triggers, RadixTrigger{
				Name: "Memory",
				Memory: &RadixHorizontalScalingMemoryTrigger{
					MetricType: autoscalingv2.UtilizationMetricType,
					Value:      int(*c.RadixHorizontalScalingResources.Memory.AverageUtilization),
				},
			})
		}
	}

	if config.MinReplicas == nil {
		config.MinReplicas = pointers.Ptr[int32](1)
	}

	// Set defaults for triggers
	if config.Triggers != nil {
		for _, trigger := range *config.Triggers {
			switch {
			case trigger.Cpu != nil:
				if trigger.Cpu.MetricType == "" {
					trigger.Cpu.MetricType = autoscalingv2.UtilizationMetricType
				}
			case trigger.Memory != nil:
				if trigger.Memory.MetricType == "" {
					trigger.Memory.MetricType = autoscalingv2.UtilizationMetricType
				}
			}
		}
	}

	if config.Triggers == nil || len(*config.Triggers) == 0 {
		config.Triggers = &[]RadixTrigger{
			{
				Name: "default-cpu",
				Cpu: &RadixHorizontalScalingCPUTrigger{
					MetricType: autoscalingv2.UtilizationMetricType,
					Value:      DefaultTargetCPUUtilizationPercentage,
				},
			},
		}
	}

	return config
}
