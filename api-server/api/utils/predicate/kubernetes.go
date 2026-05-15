package predicate

import (
	"github.com/equinor/radix-operator/api-server/api/utils/labelselector"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	secretStoreCsiManagedLabel = "secrets-store.csi.k8s.io/managed"
)

func IsPodForComponent(appName, componentName string) func(corev1.Pod) bool {
	selector := labelselector.ForComponent(appName, componentName).AsSelector()
	return func(pod corev1.Pod) bool {
		return selector.Matches(labels.Set(pod.Labels))
	}
}

func IsDeploymentForComponent(appName, componentName string) func(appsv1.Deployment) bool {
	selector := labelselector.ForComponent(appName, componentName).AsSelector()
	return func(deployment appsv1.Deployment) bool {
		return selector.Matches(labels.Set(deployment.Labels))
	}
}

func IsPodForAuxComponent(appName, componentName, auxType string) func(corev1.Pod) bool {
	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).AsSelector()
	return func(pod corev1.Pod) bool {
		return selector.Matches(labels.Set(pod.Labels))
	}
}

func IsDeploymentForAuxComponent(appName, componentName, auxType string) func(appsv1.Deployment) bool {
	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).AsSelector()
	return func(deployment appsv1.Deployment) bool {
		return selector.Matches(labels.Set(deployment.Labels))
	}
}

func IsHpaForComponent(appName, componentName string) func(autoscalingv2.HorizontalPodAutoscaler) bool {
	selector := labels.SelectorFromSet(radixlabels.Merge(radixlabels.ForApplicationName(appName), radixlabels.ForComponentName(componentName)))
	return func(hpa autoscalingv2.HorizontalPodAutoscaler) bool {
		return selector.Matches(labels.Set(hpa.Labels))
	}
}
func IsScaledObjectForComponent(appName, componentName string) func(object v1alpha1.ScaledObject) bool {
	selector := labels.SelectorFromSet(radixlabels.Merge(radixlabels.ForApplicationName(appName), radixlabels.ForComponentName(componentName)))
	return func(object v1alpha1.ScaledObject) bool {
		return selector.Matches(labels.Set(object.Labels))
	}
}

func IsSecretForSecretStoreProviderClass(secret corev1.Secret) bool {
	return labels.Set{secretStoreCsiManagedLabel: "true"}.AsSelector().Matches(labels.Set(secret.Labels))
}
