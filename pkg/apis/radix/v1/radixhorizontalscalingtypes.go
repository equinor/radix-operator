package v1

import (
	"github.com/equinor/radix-common/utils/pointers"
)

const (
	// DefaultTargetCPUUtilizationPercentage is the default target CPU utilization percentage for horizontal scaling.
	DefaultTargetCPUUtilizationPercentage = 80
)

// RadixHorizontalScaling defines configuration for horizontal pod autoscaler.
type RadixHorizontalScaling struct {
	// Defines the minimum number of replicas.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Defines the maximum number of replicas.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// PollingInterval configures how often to check each trigger on. Defaults to 30sec
	// +optional
	// +kubebuilder:validation:Minimum=15
	PollingInterval *int32 `json:"pollingInterval,omitempty"` // 30

	// CooldownPeriod to wait after the last trigger reported active before scaling the resource back to 0. Defaults to 5min
	// +optional
	// +kubebuilder:validation:Minimum=15
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty"` // 300

	// Deprecated: Use CPU and/or Memory triggers instead
	// Defines the resource usage parameters for the horizontal pod autoscaler.
	// +optional
	RadixHorizontalScalingResources *RadixHorizontalScalingResources `json:"resources,omitempty"`

	// Defines a list of triggers the component replicas will scale on. Defaults to 80% CPU.
	// +optional
	// +listType=map
	// +listMapKey=name
	Triggers []RadixHorizontalScalingTrigger `json:"triggers,omitempty"`
}

// RadixHorizontalScalingResource defines the resource usage which triggers scaling for the horizontal pod autoscaler.
type RadixHorizontalScalingResource struct {
	// Defines the resource usage which triggers scaling for the horizontal pod autoscaler.
	// +kubebuilder:validation:Minimum=1
	AverageUtilization *int32 `json:"averageUtilization"`
}

// RadixHorizontalScalingResources defines the resource usage parameters for the horizontal pod autoscaler.
type RadixHorizontalScalingResources struct {
	// Defines the CPU usage parameters for the horizontal pod autoscaler.
	// +optional
	Cpu *RadixHorizontalScalingResource `json:"cpu,omitempty"`

	// Defines the memory usage parameters for the horizontal pod autoscaler.
	// +optional
	Memory *RadixHorizontalScalingResource `json:"memory,omitempty"`
}

// RadixHorizontalScalingTrigger defines configuration for a specific trigger.
// +kubebuilder:validation:MinProperties=2
// +kubebuilder:validation:MaxProperties=2
type RadixHorizontalScalingTrigger struct {
	// Name of trigger, must be unique
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=`^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$`
	Name string `json:"name"`

	// Cpu defines a trigger based on CPU usage
	Cpu *RadixHorizontalScalingCPUTrigger `json:"cpu,omitempty"`

	// Memory defines a trigger based on memory usage
	Memory *RadixHorizontalScalingMemoryTrigger `json:"memory,omitempty"`

	// Cron defines a trigger that scales based on start and end times
	Cron *RadixHorizontalScalingCronTrigger `json:"cron,omitempty"`

	// AzureServiceBus defines a trigger that scales based on number of messages in queue
	AzureServiceBus *RadixHorizontalScalingAzureServiceBusTrigger `json:"azureServiceBus,omitempty"`

	// AzureEventHub defines a trigger that scales based on number of unprocessed events in event hub
	AzureEventHub *RadixHorizontalScalingAzureEventHubTrigger `json:"azureEventHub,omitempty"`
}

// RadixHorizontalScalingCPUTrigger defines configuration for a CPU trigger.
type RadixHorizontalScalingCPUTrigger struct {
	// Value - the target value is the average of the resource metric across all relevant pods, represented as a percentage of the requested value of the resource for the pods.
	// +kubebuilder:validation:Minimum=15
	Value int `json:"value"`
}

// RadixHorizontalScalingMemoryTrigger defines a trigger based on memory usage.
type RadixHorizontalScalingMemoryTrigger struct {
	// Value - the target value is the average of the resource metric across all relevant pods, represented as a percentage of the requested value of the resource for the pods.
	// +kubebuilder:validation:Minimum=15
	Value int `json:"value"`
}

// RadixHorizontalScalingCronTrigger defines configuration for a cron trigger.
type RadixHorizontalScalingCronTrigger struct {
	// Start is a Cron expression indicating the start of the cron schedule.
	// +kubebuilder:validation:Pattern=`^((((\d+,)+\d+|(\d+(\/|-)\d+)|\d+|\*) ?){5})$`
	Start string `json:"start"`

	// End is a Cron expression indicating the End of the cron schedule.
	// +kubebuilder:validation:Pattern=`^((((\d+,)+\d+|(\d+(\/|-)\d+)|\d+|\*) ?){5})$`
	End string `json:"end"`

	// Timezone One of the acceptable values from the IANA Time Zone Database. The list of timezones can be found at https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
	Timezone string `json:"timezone"`

	// DesiredReplicas Number of replicas to which the resource has to be scaled between the start and end of the cron schedule.
	// +kubebuilder:validation:Minimum=1
	DesiredReplicas int `json:"desiredReplicas"`
}

// RadixHorizontalScalingAzureServiceBusTrigger defines configuration for an Azure Service Bus trigger.
type RadixHorizontalScalingAzureServiceBusTrigger struct {
	// Namespace - Name of the Azure Service Bus namespace that contains your queue or topic. Required when using workload identity
	// +kubebuilder:validation:MinLength=6
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=^(([a-z][-a-z0-9]*)?[a-z0-9])?$
	Namespace string `json:"namespace"`

	// QueueName selects the target queue. QueueName wil take precedence over TopicName.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=260
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-_a-z0-9./]*)?[a-z0-9])?$
	QueueName string `json:"queueName,omitempty"`

	// TopicName selectes the target topic, requires SubscriptionName to be set.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=260
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-_a-z0-9./]*)?[a-z0-9])?$
	TopicName string `json:"topicName,omitempty"`

	// SubscriptionName is required when TopicName is set.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-_a-z0-9./]*)?[a-z0-9])?$
	SubscriptionName string `json:"subscriptionName,omitempty"`

	// ConnectionFromEnv The name of the environment variable your deployment uses to get the connection string of the Azure Service Bus namespace.
	// Ignored when used Workload Identity.
	// +optional
	// +kubebuilder:validation:MaxLength=50
	ConnectionFromEnv string `json:"connectionFromEnv,omitempty"`

	// MessageCount  - Amount of active messages in your Azure Service Bus queue or topic to scale on. Defaults to 5 messages
	// +optional
	// DesiredReplicas Number of replicas to which the resource has to be scaled between the start and end of the cron schedule.
	// +kubebuilder:validation:Minimum=1
	MessageCount *int `json:"messageCount,omitempty"`

	// ActivationMessageCount = Target value for activating the scaler. (Default: 0, Optional)
	// +optional
	// +kubebuilder:validation:Minimum=0
	ActivationMessageCount *int `json:"activationMessageCount,omitempty"`

	// Azure Service Bus requires Workload Identity configured with a ClientID
	// +optional
	Authentication RadixHorizontalScalingAuthentication `json:"authentication,omitempty"`
}

type AzureEventHubTriggerCheckpointStrategy string

const (
	// AzureEventHubTriggerCheckpointStrategyGoSdk uses the go SDK to checkpoint
	AzureEventHubTriggerCheckpointStrategyGoSdk AzureEventHubTriggerCheckpointStrategy = "goSdk"
	// AzureEventHubTriggerCheckpointStrategyBlobMetadata uses the blob metadata to checkpoint
	AzureEventHubTriggerCheckpointStrategyBlobMetadata AzureEventHubTriggerCheckpointStrategy = "blobMetadata"
	// AzureEventHubTriggerCheckpointStrategyAzureFunction uses the Azure Function checkpointing strategy.
	AzureEventHubTriggerCheckpointStrategyAzureFunction AzureEventHubTriggerCheckpointStrategy = "azureFunction"
)

// RadixHorizontalScalingAzureEventHubTrigger defines configuration for an Azure Event Hub trigger.
type RadixHorizontalScalingAzureEventHubTrigger struct {
	// EventHubNamespace The Event Hubs namespace to build FQDN like myeventhubnamespace.servicebus.windows.netname
	// +optional
	// +kubebuilder:validation:MaxLength=150
	// +kubebuilder:validation:Pattern=^(([a-z][-a-z0-9]*)?[a-z0-9])?$
	EventHubNamespace string `json:"eventHubNamespace,omitempty"`

	// EventHubNamespaceFromEnv The name of the environment variable or secret  holding the Event Hubs namespace to build FQDN like myeventhubnamespace.servicebus.windows.netname
	// It is ignored when EventHubNamespace is defined.
	// +optional
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=^(([a-zA-Z][_a-zA-Z0-9]*)?[a-zA-Z0-9])?$
	EventHubNamespaceFromEnv string `json:"eventHubNamespaceFromEnv,omitempty"`

	// EventHubName of the Azure Event Hub within Event Hub namespace
	// +optional
	// +kubebuilder:validation:MaxLength=260
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-_a-z0-9./]*)?[a-z0-9])?$
	EventHubName string `json:"eventHubName,omitempty"`

	// EventHubNameFromEnv The name of the environment variable or secret holding the Azure Event Hub name.
	// It is ignored when EventHubName is defined.
	// +optional
	// +kubebuilder:validation:MaxLength=260
	// +kubebuilder:validation:Pattern=^(([a-zA-Z][_a-zA-Z0-9]*)?[a-zA-Z0-9])?$
	EventHubNameFromEnv string `json:"eventHubNameFromEnv,omitempty"`

	// ConsumerGroup is the name of the consumer group to use when consuming events from the Event Hub. Defaults to $Default
	// +optional
	// +kubebuilder:validation:MaxLength=150
	ConsumerGroup string `json:"consumerGroup,omitempty"`

	// EventHubConnectionFromEnv The name of the environment variable or secret holding the connection string for the Event Hub. This is required when not using identity based authentication to Event Hub.
	// String should be in following format: Endpoint=sb://<event-hub-namespace>.servicebus.windows.net/;SharedAccessKeyName=<key-name>;SharedAccessKey=<key-value>;EntityPath=<event-hub-name>
	// EntityPath is optional. If it is not provided, then Name must be used to provide the name of the Azure Event Hub instance to use inside the namespace.
	// +optional
	// +kubebuilder:validation:MaxLength=50
	// Example:
	// Endpoint=sb://eventhub-namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=secretKey123;EntityPath=eventhub-name
	EventHubConnectionFromEnv string `json:"eventHubConnectionFromEnv,omitempty"`

	// StorageConnectionFromEnv The name of the environment variable or secret holding the connection string for storage account used to store checkpoint. As of now the Event Hub scaler only reads from Azure Blob Storage.
	// +optional
	// +kubebuilder:validation:MaxLength=50
	StorageConnectionFromEnv string `json:"storageConnectionFromEnv,omitempty"`

	// StorageAccount Name of the storage account used for checkpointing. If storage account is not specified when used identity based authentication to Blob Storage, the StorageConnectionFromEnv will be used.
	// It is ignored when EventHubConnectionFromEnv is defined.
	// +optional
	// +kubebuilder:validation:MaxLength=150
	StorageAccount string `json:"accountName,omitempty"`

	// Container is the name of the Blob Storage container used for checkpointing.
	// This is needed for every checkpointStrategy except of AzureFunction. With Azure Functions checkpointStrategy the Container is automatically set or overridden as azure-webjobs-eventhub.
	// It should be set to azure-webjobs-eventhub for Azure Functions using blobMetadata as checkpointStrategy.
	// +optional
	// +kubebuilder:validation:MaxLength=150
	Container string `json:"container,omitempty"`

	// CheckpointStrategy defines the strategy to use for checkpointing. Defaults to blobMetadata.
	// +optional
	// +kubebuilder:validation:Enum=goSdk;blobMetadata;azureFunction;""
	CheckpointStrategy AzureEventHubTriggerCheckpointStrategy `json:"checkpointStrategy,omitempty"`

	// UnprocessedEventThreshold Average target value to trigger scaling actions. Default: 64 events.
	// +optional
	// +kubebuilder:validation:Minimum=1
	UnprocessedEventThreshold *int `json:"unprocessedEventThreshold,omitempty"`

	// ActivationUnprocessedEventThreshold Target value for activating the scaler. Defaults to 0.
	// Learn more about activation https://keda.sh/docs/2.17/concepts/scaling-deployments/#activating-and-scaling-thresholds
	// +optional
	// +kubebuilder:validation:Minimum=0
	ActivationUnprocessedEventThreshold *int `json:"activationUnprocessedEventThreshold,omitempty"`

	// Authentication Workload Identity configured with a ClientID when used identity based authentication
	// +optional
	Authentication *RadixHorizontalScalingAuthentication `json:"authentication"`
}

// RadixHorizontalScalingAuthentication defines the authentication configuration for horizontal scaling.
type RadixHorizontalScalingAuthentication struct {
	Identity RadixHorizontalScalingRequiredIdentity `json:"identity"`
}

// RadixHorizontalScalingRequiredIdentity configuration for federation with required azure identity providers.
type RadixHorizontalScalingRequiredIdentity struct {
	// Azure identity configuration
	Azure AzureIdentity `json:"azure"`
}

// NormalizeConfig copies, migrate deprecations and add defaults to configuration
func (c *RadixHorizontalScaling) NormalizeConfig() *RadixHorizontalScaling {
	// This method could probably return a v2alpha1.RadixHorizontalScaling when infrastructure for that is ready
	if c == nil {
		return nil
	}

	config := c.DeepCopy()
	config.RadixHorizontalScalingResources = nil

	if c.RadixHorizontalScalingResources != nil && len(config.Triggers) == 0 {
		if c.RadixHorizontalScalingResources.Cpu != nil && c.RadixHorizontalScalingResources.Cpu.AverageUtilization != nil {
			config.Triggers = append(config.Triggers, RadixHorizontalScalingTrigger{
				Name: "cpu",
				Cpu: &RadixHorizontalScalingCPUTrigger{
					Value: int(*c.RadixHorizontalScalingResources.Cpu.AverageUtilization),
				},
			})
		}

		if c.RadixHorizontalScalingResources.Memory != nil && c.RadixHorizontalScalingResources.Memory.AverageUtilization != nil {
			config.Triggers = append(config.Triggers, RadixHorizontalScalingTrigger{
				Name: "memory",
				Memory: &RadixHorizontalScalingMemoryTrigger{
					Value: int(*c.RadixHorizontalScalingResources.Memory.AverageUtilization),
				},
			})
		}
	}

	if config.MinReplicas == nil {
		config.MinReplicas = pointers.Ptr[int32](1)
	}

	if len(config.Triggers) == 0 {
		config.Triggers = append(config.Triggers, RadixHorizontalScalingTrigger{
			Name: "default-cpu",
			Cpu: &RadixHorizontalScalingCPUTrigger{
				Value: DefaultTargetCPUUtilizationPercentage,
			},
		})
	}

	return config
}
