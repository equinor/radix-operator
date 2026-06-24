package radixapplication

import (
	"context"
	"errors"
	"fmt"
	"slices"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/robfig/cron/v3"
)

var validAzureEventHubTriggerCheckpoints = map[radixv1.AzureEventHubTriggerCheckpointStrategy]any{radixv1.AzureEventHubTriggerCheckpointStrategyAzureFunction: struct{}{},
	radixv1.AzureEventHubTriggerCheckpointStrategyBlobMetadata: struct{}{}, radixv1.AzureEventHubTriggerCheckpointStrategyGoSdk: struct{}{}}

func horizontalScalingValidator(ctx context.Context, app *radixv1.RadixApplication) ([]string, []error) {
	var errs []error

	for _, component := range app.Spec.Components {
		if component.HorizontalScaling != nil {
			err := validateHorizontalScalingPart(component.HorizontalScaling)
			if err != nil {
				errs = append(errs, fmt.Errorf("error validating horizontal scaling for component: %s: %w", component.Name, err))
			}
		}
		for _, envConfig := range component.EnvironmentConfig {
			if envConfig.HorizontalScaling == nil {
				continue
			}

			err := validateHorizontalScalingPart(envConfig.HorizontalScaling)
			if err != nil {
				errs = append(errs, fmt.Errorf("error validating horizontal scaling for environment %s in component %s: %w", envConfig.Environment, component.Name, err))
			}
		}
	}

	return nil, errs
}

func validateHorizontalScalingPart(config *radixv1.RadixHorizontalScaling) error {
	var errs []error

	if config.RadixHorizontalScalingResources != nil && len(config.Triggers) > 0 { //nolint:staticcheck // backward compatibility support
		errs = append(errs, ErrCombiningTriggersWithResourcesIsIllegal)
	}

	config = config.NormalizeConfig()

	if config.MaxReplicas == 0 {
		errs = append(errs, ErrMaxReplicasForHPANotSetOrZero)
	}
	if *config.MinReplicas > config.MaxReplicas {
		errs = append(errs, ErrMinReplicasGreaterThanMaxReplicas)
	}

	if *config.MinReplicas == 0 && !hasNonResourceTypeTriggers(config) {
		errs = append(errs, ErrInvalidMinimumReplicasConfigurationWithMemoryAndCPUTriggers)
	}

	if err := validateTriggerDefinition(config); err != nil {
		errs = append(errs, err)
	}

	if err := validateUniqueTriggerNames(config); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func validateUniqueTriggerNames(config *radixv1.RadixHorizontalScaling) error {
	if config == nil {
		return nil
	}

	var errs []error
	var names []string

	for _, trigger := range config.Triggers {
		if slices.Contains(names, trigger.Name) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrDuplicateTriggerName, trigger.Name))
		} else {
			names = append(names, trigger.Name)
		}
	}

	return errors.Join(errs...)
}

func validateTriggerDefinition(config *radixv1.RadixHorizontalScaling) error {
	var errs []error

	for _, trigger := range config.Triggers {
		var definitions int

		if trigger.Cpu != nil {
			definitions++
			if trigger.Cpu.Value == 0 {
				errs = append(errs, fmt.Errorf("invalid trigger %s: value must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
			}
		}
		if trigger.Memory != nil {
			definitions++
			if trigger.Memory.Value == 0 {
				errs = append(errs, fmt.Errorf("invalid trigger %s: value must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
			}
		}
		if trigger.Cron != nil {
			definitions++
			errs = append(errs, validateCronTrigger(trigger)...)
		}
		if trigger.AzureServiceBus != nil {
			definitions++
			errs = append(errs, validateAzureServiceBusTrigger(trigger)...)
		}
		if trigger.AzureEventHub != nil {
			definitions++
			errs = append(errs, validateAzureEventHubTrigger(trigger)...)
		}
		if definitions == 0 {
			errs = append(errs, fmt.Errorf("invalid trigger %s: %w", trigger.Name, ErrNoDefinitionInTrigger))
		} else if definitions > 1 {
			errs = append(errs, fmt.Errorf("invalid trigger %s: %w (found %d definitions)", trigger.Name, ErrMoreThanOneDefinitionInTrigger, definitions))
		}
	}

	return errors.Join(errs...)
}

func validateCronTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error
	if trigger.Cron.Start == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: start must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	} else if err := validateKedaCronSchedule(trigger.Cron.Start); err != nil {
		errs = append(errs, fmt.Errorf("invalid trigger %s: start is invalid: %w: %w", trigger.Name, err, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.End == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: end must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	} else if err := validateKedaCronSchedule(trigger.Cron.End); err != nil {
		errs = append(errs, fmt.Errorf("invalid trigger %s: end is invalid: %w: %w", trigger.Name, err, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.Timezone == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: timezone must be set: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.Cron.DesiredReplicas < 1 {
		errs = append(errs, fmt.Errorf("invalid trigger %s: desiredReplicas must be positive integer: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func validateAzureServiceBusTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error
	// TODO: this is only requrired when using WorkloadIdentity
	if trigger.AzureServiceBus.Namespace == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: Name of the Azure Service Bus namespace that contains your queue or topic: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.AzureServiceBus.QueueName != "" && (trigger.AzureServiceBus.TopicName != "" || trigger.AzureServiceBus.SubscriptionName != "") {
		errs = append(errs, fmt.Errorf("invalid trigger %s: queueName cannot be used with topicName or subscriptionName: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}

	if trigger.AzureServiceBus.QueueName == "" && (trigger.AzureServiceBus.TopicName == "" || trigger.AzureServiceBus.SubscriptionName == "") {
		errs = append(errs, fmt.Errorf("invalid trigger %s: both topicName and subscriptionName must be set if queueName is not used: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	if trigger.AzureServiceBus.Authentication.Identity.Azure.ClientId == "" && trigger.AzureServiceBus.ConnectionFromEnv == "" {
		errs = append(errs, fmt.Errorf("invalid trigger %s: azure workload identity or connectionFromEnv are required: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func validateAzureEventHubTrigger(trigger radixv1.RadixHorizontalScalingTrigger) []error {
	var errs []error

	if auth := trigger.AzureEventHub.Authentication; auth != nil && (*auth).Identity.Azure.ClientId != "" {
		if trigger.AzureEventHub.EventHubNamespace == "" && trigger.AzureEventHub.EventHubNamespaceFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub namespace is required when used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.EventHubName == "" && trigger.AzureEventHub.EventHubNameFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub name is required when used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.StorageAccount == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: both storage account name and storage connection are required: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
	} else {
		if trigger.AzureEventHub.EventHubConnectionFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: event hub connection string is required when not used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
		if trigger.AzureEventHub.StorageConnectionFromEnv == "" {
			errs = append(errs, fmt.Errorf("invalid trigger %s: storage account connection string when not used workload identity: %w", trigger.Name, ErrInvalidTriggerDefinition))
		}
	}

	if trigger.AzureEventHub.Container == "" && trigger.AzureEventHub.CheckpointStrategy != radixv1.AzureEventHubTriggerCheckpointStrategyAzureFunction {
		errs = append(errs, fmt.Errorf("invalid trigger %s: storage account container name is required for not azureFunction checkpointStrategy: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	if !isValidAzureEventHubTriggerCheckpoints(trigger.AzureEventHub.CheckpointStrategy) {
		errs = append(errs, fmt.Errorf("invalid trigger %s: invalid checkpoint strategy: %w", trigger.Name, ErrInvalidTriggerDefinition))
	}
	return errs
}

func isValidAzureEventHubTriggerCheckpoints(checkpointStrategy radixv1.AzureEventHubTriggerCheckpointStrategy) bool {
	if checkpointStrategy == "" {
		return true
	}
	_, ok := validAzureEventHubTriggerCheckpoints[checkpointStrategy]
	return ok
}

func validateKedaCronSchedule(schedule string) error {
	// Validate same schedule as KEDA: github.com/kedacore/keda/pkg/scalers/cron_scaler.go:71
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	return err
}

// hasNonResourceTypeTriggers returns true if atleast one non resource type triggers found
func hasNonResourceTypeTriggers(config *radixv1.RadixHorizontalScaling) bool {
	for _, trigger := range config.Triggers {
		if trigger.Cron != nil {
			return true
		}
		if trigger.AzureServiceBus != nil {
			return true
		}
		if trigger.AzureEventHub != nil {
			return true
		}
	}

	return false
}
