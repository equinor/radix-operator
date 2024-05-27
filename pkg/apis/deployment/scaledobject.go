package deployment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/rs/zerolog/log"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateScaledObject(ctx context.Context, deployComponent radixv1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()
	horizontalScaling := deployComponent.GetHorizontalScaling().NormalizeConfig()

	// Check if scaler config exists
	if horizontalScaling == nil {
		log.Ctx(ctx).Debug().Msgf("Skip creating ScaledObject %s in namespace %s: no HorizontalScaling config exists", componentName, namespace)
		return nil
	}

	if isComponentStopped(deployComponent) {
		return nil
	}

	scaler := deploy.getScalerConfig(componentName, horizontalScaling)

	auths := deploy.getTriggerAuths(componentName, horizontalScaling)
	for _, auth := range auths {
		if err := deploy.kubeutil.ApplyTriggerAuthentication(ctx, namespace, auth); err != nil {
			return err
		}
	}

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

		err = deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, hpa.Name, metav1.DeleteOptions{})
		if err != nil && errors.IsNotFound(err) {
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
			err = deploy.kubeutil.DeleteScaledObject(ctx, scaler)
			if err != nil {
				return err
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
			Advanced:        &kedav1.AdvancedConfig{RestoreToOriginalReplicaCount: true},
			ScaleTargetRef: &kedav1.ScaleTarget{
				Kind:       "Deployment",
				Name:       componentName,
				APIVersion: "apps/radixv1",
			},
			Triggers: triggers,
		},
	}

	return scaler
}

func getScalingTriggers(componentName string, config *radixv1.RadixHorizontalScaling) []kedav1.ScaleTriggers {
	var triggers []kedav1.ScaleTriggers

	if config == nil || config.Triggers == nil {
		return triggers
	}

	for _, trigger := range config.Triggers {
		switch {
		case trigger.Cpu != nil:
			triggers = append(triggers, kedav1.ScaleTriggers{
				Name:       trigger.Name,
				Type:       "cpu",
				MetricType: trigger.Cpu.MetricType,
				Metadata: map[string]string{
					"value": strconv.Itoa(trigger.Cpu.Value),
				},
			})
		case trigger.Memory != nil:
			triggers = append(triggers, kedav1.ScaleTriggers{
				Name:       trigger.Name,
				Type:       "memory",
				MetricType: autoscalingv2.UtilizationMetricType,
				Metadata: map[string]string{
					"value": strconv.Itoa(trigger.Memory.Value),
				},
			})
		case trigger.Cron != nil:
			triggers = append(triggers, kedav1.ScaleTriggers{
				Name: trigger.Name,
				Type: "cron",
				Metadata: map[string]string{
					"start":           trigger.Cron.Start,
					"end":             trigger.Cron.End,
					"timezone":        trigger.Cron.Timezone,
					"desiredReplicas": strconv.Itoa(trigger.Cron.DesiredReplicas),
				},
			})
		case trigger.AzureServiceBus != nil:
			metadata := map[string]string{
				"namespace": trigger.AzureServiceBus.Namespace,
			}

			if trigger.AzureServiceBus.QueueName != nil {
				metadata["queueName"] = *trigger.AzureServiceBus.QueueName
			}
			if trigger.AzureServiceBus.TopicName != nil && trigger.AzureServiceBus.SubscriptionName != nil {
				metadata["topicName"] = *trigger.AzureServiceBus.TopicName
				metadata["subscriptionName"] = *trigger.AzureServiceBus.SubscriptionName
			}
			if trigger.AzureServiceBus.MessageCount != nil {
				metadata["messageCount"] = strconv.Itoa(*trigger.AzureServiceBus.MessageCount)
			}
			if trigger.AzureServiceBus.ActivationMessageCount != nil {
				metadata["activationMessageCount"] = strconv.Itoa(*trigger.AzureServiceBus.ActivationMessageCount)
			}

			triggers = append(triggers, kedav1.ScaleTriggers{
				Name:     trigger.Name,
				Type:     "azure-servicebus",
				Metadata: metadata,
				AuthenticationRef: &kedav1.AuthenticationRef{
					Name: utils.GetTriggerAuthenticationName(componentName, trigger.Name),
					Kind: "TriggerAuthentication",
				},
			})
		}
	}

	return triggers
}

func (deploy *Deployment) getTriggerAuths(componentName string, config *radixv1.RadixHorizontalScaling) []kedav1.TriggerAuthentication {
	var auths []kedav1.TriggerAuthentication

	if config == nil || config.Triggers == nil {
		return auths
	}

	for _, trigger := range config.Triggers {
		switch {
		case trigger.AzureServiceBus != nil:
			auths = append(auths, kedav1.TriggerAuthentication{
				ObjectMeta: metav1.ObjectMeta{
					Name: utils.GetTriggerAuthenticationName(componentName, trigger.Name),
					Labels: map[string]string{
						kube.RadixAppLabel:       deploy.radixDeployment.Spec.AppName,
						kube.RadixComponentLabel: componentName,
						kube.RadixTriggerLabel:   trigger.Name,
					},
					OwnerReferences: []metav1.OwnerReference{
						getOwnerReferenceOfDeployment(deploy.radixDeployment),
					},
				},
				Spec: kedav1.TriggerAuthenticationSpec{
					PodIdentity: &kedav1.AuthPodIdentity{
						Provider:   "azure-workload",
						IdentityID: &trigger.AzureServiceBus.Authentication.Identity.Azure.ClientId,
					},
				},
			})
		}
	}

	return auths
}

func (deploy *Deployment) deleteScaledObjectIfExists(ctx context.Context, componentName string) error {
	namespace := deploy.radixDeployment.GetNamespace()

	scalers, err := deploy.kubeutil.ListScaledObjectWithSelector(ctx, namespace, labels.ForComponentName(componentName).String())
	if err != nil {
		return fmt.Errorf("failed to list ScaledObject: %w", err)
	}

	err = deploy.kubeutil.DeleteScaledObject(ctx, scalers...)
	if err != nil && errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete existing ScaledObject: %w", err)
	}

	return nil
}
