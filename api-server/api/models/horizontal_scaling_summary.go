package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

func buildHpaSummary(appName string, component radixv1.RadixCommonDeployComponent, hpaList []autoscalingv2.HorizontalPodAutoscaler, scalerList []v1alpha1.ScaledObject) *deploymentModels.HorizontalScalingSummary {
	scalingConfig := component.GetHorizontalScaling()
	if scalingConfig == nil {
		return nil
	}

	var (
		kedaScaler *v1alpha1.ScaledObject
		hpa        *autoscalingv2.HorizontalPodAutoscaler
	)
	if v, ok := slice.FindFirst(scalerList, predicate.IsScaledObjectForComponent(appName, component.GetName())); ok {
		kedaScaler = &v
	}

	if kedaScaler != nil {
		if v, ok := slice.FindFirst(hpaList, func(s autoscalingv2.HorizontalPodAutoscaler) bool { return s.Name == kedaScaler.Status.HpaName }); ok {
			hpa = &v
		}
	}

	currentUtilization, hasCurrentUtilization := resolveHorizontalScalingTriggersCurrentState(*scalingConfig, kedaScaler, hpa)
	triggers := []deploymentModels.HorizontalScalingSummaryTrigger{}

	for triggerConfigIndex, triggerConfig := range scalingConfig.Triggers {
		trigger := deploymentModels.HorizontalScalingSummaryTrigger{
			Name:              triggerConfig.Name,
			Type:              triggerConfig.Type(),
			Identity:          buildIdentityForHorizontalScalingTrigger(triggerConfig),
			TargetUtilization: resolveHorizontalScalingTargetUtilization(triggerConfig),
		}

		if hasCurrentUtilization {
			trigger.CurrentUtilization = currentUtilization[triggerConfigIndex].utilization
			trigger.Error = currentUtilization[triggerConfigIndex].errorMsg
		}

		triggers = append(triggers, trigger)
	}

	hpaSummary := deploymentModels.HorizontalScalingSummary{
		MinReplicas:     scalingConfig.MinReplicas,
		MaxReplicas:     &scalingConfig.MaxReplicas,
		CooldownPeriod:  scalingConfig.CooldownPeriod,
		PollingInterval: scalingConfig.PollingInterval,
		Triggers:        triggers,
	}

	if hpa != nil {
		hpaSummary.CurrentReplicas, hpaSummary.DesiredReplicas = hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas
	}

	return &hpaSummary
}

type horizontalScalingCurrentState struct {
	utilization string
	errorMsg    string
}

func resolveHorizontalScalingTriggersCurrentState(scalingConfig radixv1.RadixHorizontalScaling, kedaScaler *v1alpha1.ScaledObject, hpa *autoscalingv2.HorizontalPodAutoscaler) ([]horizontalScalingCurrentState, bool) {
	if kedaScaler == nil || hpa == nil {
		return nil, false
	}

	if len(kedaScaler.Spec.Triggers) != len(kedaScaler.Status.ResourceMetricNames)+len(kedaScaler.Status.ExternalMetricNames) {
		return nil, false
	}

	if len(kedaScaler.Spec.Triggers) != len(hpa.Status.CurrentMetrics) {
		return nil, false
	}

	if len(scalingConfig.Triggers) != len(kedaScaler.Spec.Triggers) {
		return nil, false
	}

	for triggerConfigIndex, triggerConfig := range scalingConfig.Triggers {
		if kedaScaler.Spec.Triggers[triggerConfigIndex].Name != triggerConfig.Name || kedaScaler.Spec.Triggers[triggerConfigIndex].Type != triggerConfig.Type() {
			return nil, false
		}
	}

	triggerStates := make([]horizontalScalingCurrentState, len(kedaScaler.Spec.Triggers))

	var externalMetricsPos, resourceMetricsPos int
	for kedaTriggerIndex, kedaTrigger := range kedaScaler.Spec.Triggers {
		if externalMetricPrefix := fmt.Sprintf("s%v-%s-", kedaTriggerIndex, kedaTrigger.Type); externalMetricsPos < len(kedaScaler.Status.ExternalMetricNames) && strings.HasPrefix(kedaScaler.Status.ExternalMetricNames[externalMetricsPos], externalMetricPrefix) {
			externalMetricName := kedaScaler.Status.ExternalMetricNames[externalMetricsPos]

			value, ok := getHorizontalScalingCurrentUtilization(hpa, externalMetricsPos, externalMetricName, autoscalingv2.ExternalMetricSourceType)
			if !ok {
				return nil, false
			}

			var errorMsg string
			if health, ok := kedaScaler.Status.Health[externalMetricName]; ok && health.Status != "Happy" {
				errorMsg = fmt.Sprintf("Number of failures: %d", *health.NumberOfFailures)
			}

			triggerStates[kedaTriggerIndex] = horizontalScalingCurrentState{utilization: value, errorMsg: errorMsg}

			externalMetricsPos++
		} else if resourceMetricsPos < len(kedaScaler.Status.ResourceMetricNames) && kedaScaler.Status.ResourceMetricNames[resourceMetricsPos] == kedaTrigger.Type {
			value, ok := getHorizontalScalingCurrentUtilization(hpa, len(kedaScaler.Status.ExternalMetricNames)+resourceMetricsPos, kedaScaler.Status.ResourceMetricNames[resourceMetricsPos], autoscalingv2.ResourceMetricSourceType)
			if !ok {
				return nil, false
			}

			triggerStates[kedaTriggerIndex] = horizontalScalingCurrentState{utilization: value}

			resourceMetricsPos++
		} else {
			return nil, false
		}
	}

	return triggerStates, true
}

func getHorizontalScalingCurrentUtilization(hpa *autoscalingv2.HorizontalPodAutoscaler, index int, metricName string, metricType autoscalingv2.MetricSourceType) (string, bool) {
	if len(hpa.Status.CurrentMetrics) <= index {
		return "", false
	}

	currentMetric := hpa.Status.CurrentMetrics[index]

	if currentMetric.Type != metricType {
		return "", currentMetric.Type == ""
	}

	switch metricType {
	case autoscalingv2.ExternalMetricSourceType:
		if currentMetric.External == nil || currentMetric.External.Metric.Name != metricName {
			return "", false
		}
		if currentMetric.External.Current.AverageValue == nil {
			return "", true
		}
		return currentMetric.External.Current.AverageValue.String(), true
	case autoscalingv2.ResourceMetricSourceType:
		if currentMetric.Resource == nil || string(currentMetric.Resource.Name) != metricName {
			return "", false
		}
		if currentMetric.Resource.Current.AverageUtilization == nil {
			return "", true
		}
		return strconv.Itoa(int(*currentMetric.Resource.Current.AverageUtilization)), true
	}

	return "", true
}

func resolveHorizontalScalingTargetUtilization(trigger radixv1.RadixHorizontalScalingTrigger) string {
	switch trigger.Type() {
	case "cpu":
		return strconv.Itoa(trigger.Cpu.Value)
	case "memory":
		return strconv.Itoa(trigger.Memory.Value)
	case "cron":
		return strconv.Itoa(trigger.Cron.DesiredReplicas)
	case "azure-servicebus":
		if trigger.AzureServiceBus.MessageCount == nil {
			return "5"
		}
		return strconv.Itoa(*trigger.AzureServiceBus.MessageCount)
	case "azure-eventhub":
		if trigger.AzureEventHub.UnprocessedEventThreshold == nil {
			return "64"
		}
		return strconv.Itoa(*trigger.AzureEventHub.UnprocessedEventThreshold)
	}

	return ""
}

func buildIdentityForHorizontalScalingTrigger(trigger radixv1.RadixHorizontalScalingTrigger) *deploymentModels.Identity {
	var auth *radixv1.RadixHorizontalScalingAuthentication

	switch {
	case trigger.AzureEventHub != nil:
		auth = trigger.AzureEventHub.Authentication
	case trigger.AzureServiceBus != nil && trigger.AzureServiceBus.Authentication.Identity.Azure.ClientId != "":
		auth = &trigger.AzureServiceBus.Authentication
	}

	if auth == nil || auth.Identity.Azure.ClientId == "" {
		return nil
	}

	return &deploymentModels.Identity{
		Azure: &deploymentModels.AzureIdentity{
			ClientId:           auth.Identity.Azure.ClientId,
			ServiceAccountName: "keda-operator",
			Namespace:          "keda",
		},
	}
}
