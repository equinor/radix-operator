package models

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/horizontalscaling"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

var triggerIndexRegex = regexp.MustCompile(`^s(\d+)-`)

func GetHpaSummary(appName, componentName string, hpaList []autoscalingv2.HorizontalPodAutoscaler, scalerList []v1alpha1.ScaledObject) *deploymentModels.HorizontalScalingSummary {
	scaler, ok := slice.FindFirst(scalerList, predicate.IsScaledObjectForComponent(appName, componentName))
	if !ok {
		return nil
	}
	hpa, ok := slice.FindFirst(hpaList, func(s autoscalingv2.HorizontalPodAutoscaler) bool {
		return s.Name == scaler.Status.HpaName
	})
	if !ok {
		return nil
	}

	currentCpuUtil, targetCpuUtil := getHpaMetrics(&hpa, corev1.ResourceCPU)
	currentMemoryUtil, targetMemoryUtil := getHpaMetrics(&hpa, corev1.ResourceMemory)

	var triggers []deploymentModels.HorizontalScalingSummaryTriggerStatus

	// ResourceMetricNames lists resource types, not metric names
	for _, resourceType := range scaler.Status.ResourceMetricNames {
		var trigger v1alpha1.ScaleTriggers

		if trigger, ok = slice.FindFirst(scaler.Spec.Triggers, func(t v1alpha1.ScaleTriggers) bool {
			return t.Type == resourceType
		}); !ok {
			continue
		}

		triggers = append(triggers, getResourceMetricStatus(hpa, trigger))
	}

	for _, triggerName := range scaler.Status.ExternalMetricNames {
		match := triggerIndexRegex.FindStringSubmatch(triggerName)
		if len(match) != 2 {
			continue
		}
		index, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		trigger := scaler.Spec.Triggers[index]
		triggers = append(triggers, getExternalMetricStatus(hpa, triggerName, scaler, trigger))
	}

	hpaSummary := deploymentModels.HorizontalScalingSummary{
		MinReplicas:                        scaler.Spec.MinReplicaCount,
		MaxReplicas:                        scaler.Spec.MaxReplicaCount,
		CooldownPeriod:                     scaler.Spec.CooldownPeriod,
		PollingInterval:                    scaler.Spec.PollingInterval,
		CurrentCPUUtilizationPercentage:    currentCpuUtil,
		TargetCPUUtilizationPercentage:     targetCpuUtil,
		CurrentMemoryUtilizationPercentage: currentMemoryUtil,
		TargetMemoryUtilizationPercentage:  targetMemoryUtil,
		Triggers:                           triggers,
		CurrentReplicas:                    hpa.Status.CurrentReplicas,
		DesiredReplicas:                    hpa.Status.DesiredReplicas,
	}
	return &hpaSummary
}

func getResourceMetricStatus(hpa autoscalingv2.HorizontalPodAutoscaler, trigger v1alpha1.ScaleTriggers) deploymentModels.HorizontalScalingSummaryTriggerStatus {
	var current string
	if metricStatus, ok := slice.FindFirst(hpa.Status.CurrentMetrics, func(s autoscalingv2.MetricStatus) bool {
		return s.Resource != nil && s.Resource.Name.String() == trigger.Type
	}); ok && metricStatus.Resource != nil {
		current = fmt.Sprintf("%d", *metricStatus.Resource.Current.AverageUtilization)
	}

	status := deploymentModels.HorizontalScalingSummaryTriggerStatus{
		Name:               trigger.Name,
		CurrentUtilization: current,
		TargetUtilization:  trigger.Metadata["value"],
		Type:               trigger.Type,
		Error:              "",
	}
	return status
}

func getExternalMetricStatus(hpa autoscalingv2.HorizontalPodAutoscaler, triggerName string, scaler v1alpha1.ScaledObject, trigger v1alpha1.ScaleTriggers) deploymentModels.HorizontalScalingSummaryTriggerStatus {
	var current, target, errStr string

	if metricStatus, ok := slice.FindFirst(hpa.Status.CurrentMetrics, func(s autoscalingv2.MetricStatus) bool {
		return s.External != nil && s.External.Metric.Name == triggerName
	}); ok && metricStatus.External != nil {
		current = metricStatus.External.Current.AverageValue.String()
	}

	if health, ok := scaler.Status.Health[triggerName]; ok && health.Status != "Happy" {
		errStr = fmt.Sprintf("Number of failures: %d", *health.NumberOfFailures)
	}

	switch trigger.Type {
	case "cron":
		target = trigger.Metadata["desiredReplicas"]
	case "azure-servicebus":
		target = trigger.Metadata["messageCount"]
	case "azure-eventhub":
		target = trigger.Metadata["unprocessedEventThreshold"]
	}

	status := deploymentModels.HorizontalScalingSummaryTriggerStatus{
		Name:               trigger.Name,
		CurrentUtilization: current,
		TargetUtilization:  target,
		Type:               trigger.Type,
		Error:              errStr,
	}
	return status
}

func getHpaMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) (*int32, *int32) {
	currentResourceUtil := getHpaCurrentMetric(hpa, resourceName)

	// find resource utilization target
	var targetResourceUtil *int32
	targetResourceMetric := horizontalscaling.GetHpaMetric(hpa, resourceName)
	if targetResourceMetric != nil {
		targetResourceUtil = targetResourceMetric.Resource.Target.AverageUtilization
	}
	return currentResourceUtil, targetResourceUtil
}

func getHpaCurrentMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) *int32 {
	for _, metric := range hpa.Status.CurrentMetrics {
		if metric.Resource != nil && metric.Resource.Name == resourceName {
			return metric.Resource.Current.AverageUtilization
		}
	}
	return nil
}
