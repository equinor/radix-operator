package deployment

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateScaledObject(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()

	if deployComponent.GetReplicasOverride() != nil {
		log.Ctx(ctx).Debug().Msgf("Skip creating ScaledObject %s in namespace %s: manuall override is set", componentName, namespace)
		return nil
	}

	// Check if scaler config exists
	horizontalScaling := deployComponent.GetHorizontalScaling().NormalizeConfig()
	if horizontalScaling == nil {
		log.Ctx(ctx).Debug().Msgf("Skip creating ScaledObject %s in namespace %s: no HorizontalScaling config exists", componentName, namespace)
		return nil
	}

	if isComponentStopped(deployComponent) {
		return nil
	}

	auths := deploy.getTriggerAuths(componentName, horizontalScaling)
	for _, auth := range auths {
		if err := deploy.kubeutil.ApplyTriggerAuthentication(ctx, namespace, auth); err != nil {
			return err
		}
	}

	scaler := deploy.getScalerConfig(componentName, horizontalScaling)
	return deploy.kubeutil.ApplyScaledObject(ctx, namespace, scaler)
}

func (deploy *Deployment) garbageCollectDeprecatedHPAs(ctx context.Context) error {
	namespace := deploy.radixDeployment.GetNamespace()
	hpas, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(deploy.registration.Name).String()})

	if err != nil {
		return err
	}

	for _, hpa := range hpas.Items {
		// If owner reference is *not* RadixDeployment, skip deleting it
		if len(hpa.OwnerReferences) == 0 {
			continue
		}
		if owner := hpa.OwnerReferences[0]; owner.Kind != radixv1.KindRadixDeployment {
			continue
		}

		if err = deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, hpa.Name, metav1.DeleteOptions{}); err != nil && errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectScalersNoLongerInSpec(ctx context.Context) error {
	namespace := deploy.radixDeployment.GetNamespace()
	scalers, err := deploy.kubeutil.ListScaledObject(ctx, namespace)

	if err != nil {
		return err
	}

	for _, scaler := range scalers {
		componentName, ok := RadixComponentNameFromComponentLabel(scaler)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			if err = deploy.kubeutil.DeleteScaledObject(ctx, scaler); err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectTriggerAuthsNoLongerInSpec(ctx context.Context) error {
	namespace := deploy.radixDeployment.GetNamespace()

	for _, component := range deploy.radixDeployment.Spec.Components {
		horizontalScaling := component.HorizontalScaling.NormalizeConfig()
		targetAuths := deploy.getTriggerAuths(component.Name, horizontalScaling)
		currentAuths, err := deploy.kubeutil.ListTriggerAuthenticationsWithSelector(ctx, namespace, labels.ForComponentName(component.Name).String())
		if err != nil {
			return err
		}

		for _, currentAuth := range currentAuths {
			_, ok := RadixComponentNameFromComponentLabel(currentAuth)
			if !ok {
				continue
			}

			found := slices.ContainsFunc(targetAuths, func(item kedav1.TriggerAuthentication) bool {
				return item.Name == currentAuth.Name
			})

			if !found {
				if err = deploy.kubeutil.DeleteTriggerAuthentication(ctx, currentAuth); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (deploy *Deployment) getScalerConfig(componentName string, config *radixv1.RadixHorizontalScaling) *kedav1.ScaledObject {
	appName := deploy.radixDeployment.Spec.AppName
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}

	triggers := getScalingTriggers(componentName, config)

	scaler := &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: kedav1.ScaledObjectSpec{
			MinReplicaCount: config.MinReplicas,
			MaxReplicaCount: pointers.Ptr(config.MaxReplicas),
			PollingInterval: config.PollingInterval,
			CooldownPeriod:  config.CooldownPeriod,
			Advanced:        &kedav1.AdvancedConfig{RestoreToOriginalReplicaCount: false},
			ScaleTargetRef: &kedav1.ScaleTarget{
				Kind:       "Deployment",
				Name:       componentName,
				APIVersion: appsv1.SchemeGroupVersion.Identifier(),
			},
			Triggers: triggers,
		},
	}

	return scaler
}

func getScalingTriggers(componentName string, config *radixv1.RadixHorizontalScaling) []kedav1.ScaleTriggers {
	var triggers []kedav1.ScaleTriggers

	if config == nil {
		return triggers
	}

	for _, trigger := range config.Triggers {
		switch {
		case trigger.Cpu != nil:
			triggers = append(triggers, getCpuTrigger(trigger))
		case trigger.Memory != nil:
			triggers = append(triggers, getMemoryTrigger(trigger))
		case trigger.Cron != nil:
			triggers = append(triggers, getCronTrigger(trigger))
		case trigger.AzureServiceBus != nil:
			triggers = append(triggers, getAzureServiceBus(componentName, trigger))
		case trigger.AzureEventHub != nil:
			triggers = append(triggers, getAzureEventHub(componentName, trigger))
		}
	}
	return triggers
}

func getCpuTrigger(trigger radixv1.RadixHorizontalScalingTrigger) kedav1.ScaleTriggers {
	return kedav1.ScaleTriggers{
		Name:       trigger.Name,
		Type:       "cpu",
		MetricType: autoscalingv2.UtilizationMetricType,
		Metadata: map[string]string{
			"value": strconv.Itoa(trigger.Cpu.Value),
		},
	}
}

func getMemoryTrigger(trigger radixv1.RadixHorizontalScalingTrigger) kedav1.ScaleTriggers {
	return kedav1.ScaleTriggers{
		Name:       trigger.Name,
		Type:       "memory",
		MetricType: autoscalingv2.UtilizationMetricType,
		Metadata: map[string]string{
			"value": strconv.Itoa(trigger.Memory.Value),
		},
	}
}

func getCronTrigger(trigger radixv1.RadixHorizontalScalingTrigger) kedav1.ScaleTriggers {
	return kedav1.ScaleTriggers{
		Name: trigger.Name,
		Type: "cron",
		Metadata: map[string]string{
			"start":           trigger.Cron.Start,
			"end":             trigger.Cron.End,
			"timezone":        trigger.Cron.Timezone,
			"desiredReplicas": strconv.Itoa(trigger.Cron.DesiredReplicas),
		},
	}
}

func getAzureServiceBus(componentName string, trigger radixv1.RadixHorizontalScalingTrigger) kedav1.ScaleTriggers {
	metadata := map[string]string{}

	scaleTriggers := kedav1.ScaleTriggers{
		Name: trigger.Name,
		Type: "azure-servicebus",
	}

	if auth := trigger.AzureServiceBus.Authentication.Identity.Azure; auth.ClientId != "" {
		scaleTriggers.AuthenticationRef = &kedav1.AuthenticationRef{
			Name: utils.GetTriggerAuthenticationName(componentName, trigger.Name),
			Kind: "TriggerAuthentication",
		}
	} else {
		if trigger.AzureServiceBus.ConnectionFromEnv != "" {
			metadata["connectionFromEnv"] = trigger.AzureServiceBus.ConnectionFromEnv
		}
	}

	if trigger.AzureServiceBus.Namespace != "" {
		metadata["namespace"] = trigger.AzureServiceBus.Namespace
	}
	if trigger.AzureServiceBus.QueueName != "" {
		metadata["queueName"] = trigger.AzureServiceBus.QueueName
	}
	if trigger.AzureServiceBus.TopicName != "" {
		metadata["topicName"] = trigger.AzureServiceBus.TopicName
	}
	if trigger.AzureServiceBus.SubscriptionName != "" {
		metadata["subscriptionName"] = trigger.AzureServiceBus.SubscriptionName
	}
	if trigger.AzureServiceBus.MessageCount != nil {
		metadata["messageCount"] = strconv.Itoa(*trigger.AzureServiceBus.MessageCount)
	}
	if trigger.AzureServiceBus.ActivationMessageCount != nil {
		metadata["activationMessageCount"] = strconv.Itoa(*trigger.AzureServiceBus.ActivationMessageCount)
	}
	scaleTriggers.Metadata = metadata
	return scaleTriggers
}

func getAzureEventHub(componentName string, trigger radixv1.RadixHorizontalScalingTrigger) kedav1.ScaleTriggers {
	metadata := map[string]string{}

	scaleTriggers := kedav1.ScaleTriggers{
		Name: trigger.Name,
		Type: "azure-eventhub",
	}

	if auth := trigger.AzureEventHub.Authentication; auth != nil && (*auth).Identity.Azure.ClientId != "" {
		scaleTriggers.AuthenticationRef = &kedav1.AuthenticationRef{
			Name: utils.GetTriggerAuthenticationName(componentName, trigger.Name),
			Kind: "TriggerAuthentication",
		}
		if trigger.AzureEventHub.EventHubNamespace != "" {
			metadata["eventHubNamespace"] = trigger.AzureEventHub.EventHubNamespace
		} else if trigger.AzureEventHub.EventHubNamespaceFromEnv != "" {
			metadata["eventHubNamespaceFromEnv"] = trigger.AzureEventHub.EventHubNamespaceFromEnv
		}
		if trigger.AzureEventHub.StorageAccount != "" {
			metadata["storageAccountName"] = trigger.AzureEventHub.StorageAccount
		}
	} else {
		if trigger.AzureEventHub.EventHubConnectionFromEnv != "" {
			metadata["connectionFromEnv"] = trigger.AzureEventHub.EventHubConnectionFromEnv
		}
		if trigger.AzureEventHub.StorageConnectionFromEnv != "" {
			metadata["storageConnectionFromEnv"] = trigger.AzureEventHub.StorageConnectionFromEnv
		}
	}

	// Required when used Workload Identity or when EventHub Connection string has no suffix EntityPath=eventhub-name
	if trigger.AzureEventHub.EventHubName != "" {
		metadata["eventHubName"] = trigger.AzureEventHub.EventHubName
	} else if trigger.AzureEventHub.EventHubNameFromEnv != "" {
		metadata["eventHubNameFromEnv"] = trigger.AzureEventHub.EventHubNameFromEnv
	}

	if trigger.AzureEventHub.ConsumerGroup != "" {
		metadata["consumerGroup"] = trigger.AzureEventHub.ConsumerGroup
	}

	if messageCount := trigger.AzureEventHub.UnprocessedEventThreshold; messageCount != nil {
		metadata["unprocessedEventThreshold"] = strconv.Itoa(*messageCount)
	}
	if trigger.AzureEventHub.ActivationUnprocessedEventThreshold != nil {
		metadata["activationUnprocessedEventThreshold"] = strconv.Itoa(*trigger.AzureEventHub.ActivationUnprocessedEventThreshold)
	}

	checkpointStrategy := getAzureEventHubCheckpointStrategy(trigger)
	metadata["checkpointStrategy"] = string(checkpointStrategy)

	metadata["blobContainer"] = trigger.AzureEventHub.Container
	// With Azure Functions checkpointStrategy the Container is automatically set or overridden as azure-webjobs-eventhub.
	if checkpointStrategy == radixv1.AzureEventHubTriggerCheckpointStrategyAzureFunction {
		metadata["blobContainer"] = "azure-webjobs-eventhub"
	}

	scaleTriggers.Metadata = metadata
	return scaleTriggers
}

func getAzureEventHubCheckpointStrategy(trigger radixv1.RadixHorizontalScalingTrigger) radixv1.AzureEventHubTriggerCheckpointStrategy {
	if trigger.AzureEventHub.CheckpointStrategy == "" {
		return radixv1.AzureEventHubTriggerCheckpointStrategyBlobMetadata
	}
	return trigger.AzureEventHub.CheckpointStrategy
}

func (deploy *Deployment) getTriggerAuths(componentName string, config *radixv1.RadixHorizontalScaling) []kedav1.TriggerAuthentication {
	var auths []kedav1.TriggerAuthentication

	if config == nil {
		return auths
	}

	for _, trigger := range config.Triggers {
		switch {
		case trigger.AzureServiceBus != nil:
			if auth := trigger.AzureServiceBus.Authentication; auth.Identity.Azure.ClientId != "" {
				auths = append(auths, deploy.getTriggerAuthentication(componentName, trigger.Name, trigger.AzureServiceBus.Authentication))
			}
		case trigger.AzureEventHub != nil:
			if auth := trigger.AzureEventHub.Authentication; auth != nil {
				auths = append(auths, deploy.getTriggerAuthentication(componentName, trigger.Name, *auth))
			}
		}
	}

	return auths
}

func (deploy *Deployment) getTriggerAuthentication(componentName, triggerName string, authentication radixv1.RadixHorizontalScalingAuthentication) kedav1.TriggerAuthentication {
	return kedav1.TriggerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.GetTriggerAuthenticationName(componentName, triggerName),
			Labels: map[string]string{
				kube.RadixAppLabel:       deploy.radixDeployment.Spec.AppName,
				kube.RadixComponentLabel: componentName,
				kube.RadixTriggerLabel:   triggerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReferenceOfDeployment(deploy.radixDeployment),
			},
		},
		Spec: kedav1.TriggerAuthenticationSpec{
			PodIdentity: &kedav1.AuthPodIdentity{
				Provider:   "azure-workload",
				IdentityID: &authentication.Identity.Azure.ClientId,
			},
		},
	}
}

func (deploy *Deployment) deleteScaledObjectIfExists(ctx context.Context, componentName string) error {
	namespace := deploy.radixDeployment.GetNamespace()

	scalers, err := deploy.kubeutil.ListScaledObjectWithSelector(ctx, namespace, labels.ForComponentName(componentName).String())
	if err != nil {
		return fmt.Errorf("failed to list ScaledObject: %w", err)
	}

	if err = deploy.kubeutil.DeleteScaledObject(ctx, scalers...); err != nil && errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete existing ScaledObject: %w", err)
	}
	return nil
}

func (deploy *Deployment) deleteTargetAuthenticationIfExists(ctx context.Context, componentName string) error {
	namespace := deploy.radixDeployment.GetNamespace()

	auths, err := deploy.kubeutil.ListTriggerAuthenticationsWithSelector(ctx, namespace, labels.ForComponentName(componentName).String())
	if err != nil {
		return fmt.Errorf("failed to list TargetAuthentication: %w", err)
	}

	if err = deploy.kubeutil.DeleteTriggerAuthentication(ctx, auths...); err != nil && errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete existing TargetAuthentication: %w", err)
	}

	return nil
}
