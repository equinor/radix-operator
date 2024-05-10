package utils

import (
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
)

// GetHpaMetric returns the metric spec for the given resource name or nil if not found
func GetHpaMetric(hpa *v2.HorizontalPodAutoscaler, resourceName v1.ResourceName) *v2.MetricSpec {
	for _, metric := range hpa.Spec.Metrics {
		if metric.Resource != nil && metric.Resource.Name == resourceName {
			return &metric
		}
	}
	return nil
}

// GetScaleTrigger returns the trigger spec for the given name  and type or nil if not found
func GetScaleTrigger(hpa *v1alpha1.ScaledObject, triggerName, triggerType string) *v1alpha1.ScaleTriggers {
	for _, trigger := range hpa.Spec.Triggers {
		if trigger.Type == triggerType && trigger.Name == triggerName {
			return &trigger
		}
	}
	return nil
}
