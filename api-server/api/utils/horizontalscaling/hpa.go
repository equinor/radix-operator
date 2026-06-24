package horizontalscaling

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

func GetHpaMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) *autoscalingv2.MetricSpec {
	for _, metric := range hpa.Spec.Metrics {
		if metric.Resource != nil && metric.Resource.Name == resourceName {
			return &metric
		}
	}
	return nil
}
