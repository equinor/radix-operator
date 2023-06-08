package utils

import (
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
