package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const targetCPUUtilizationPercentage int32 = 80

func (deploy *Deployment) createOrUpdateHPA(deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()
	horizontalScaling := deployComponent.GetHorizontalScaling()

	// Check if hpa config exists
	if horizontalScaling == nil {
		deploy.logger.Debug().Msgf("Skip creating HorizontalPodAutoscaler %s in namespace %s: no HorizontalScaling config exists", componentName, namespace)
		return nil
	}

	if isComponentStopped(deployComponent) {
		return nil
	}

	var memoryTarget, cpuTarget *int32
	if horizontalScaling.RadixHorizontalScalingResources != nil {
		if horizontalScaling.RadixHorizontalScalingResources.Memory != nil {
			memoryTarget = horizontalScaling.RadixHorizontalScalingResources.Memory.AverageUtilization
		}

		if horizontalScaling.RadixHorizontalScalingResources.Cpu != nil {
			cpuTarget = horizontalScaling.RadixHorizontalScalingResources.Cpu.AverageUtilization
		}
	}

	if memoryTarget == nil && cpuTarget == nil {
		cpuTarget = numbers.Int32Ptr(targetCPUUtilizationPercentage)
	}

	hpa := deploy.getHPAConfig(componentName, horizontalScaling.MinReplicas, horizontalScaling.MaxReplicas, cpuTarget, memoryTarget)

	deploy.logger.Debug().Msgf("Creating HorizontalPodAutoscaler object %s in namespace %s", componentName, namespace)
	createdHPA, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		deploy.logger.Debug().Msgf("HorizontalPodAutoscaler object %s already exists in namespace %s, updating the object now", componentName, namespace)
		updatedHPA, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update HorizontalPodAutoscaler object: %v", err)
		}
		deploy.logger.Debug().Msgf("Updated HorizontalPodAutoscaler: %s in namespace %s", updatedHPA.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to create HorizontalPodAutoscaler object: %v", err)
	}
	deploy.logger.Debug().Msgf("Created HorizontalPodAutoscaler: %s in namespace %s", createdHPA.Name, namespace)
	return nil
}

func (deploy *Deployment) garbageCollectHPAsNoLongerInSpec() error {
	namespace := deploy.radixDeployment.GetNamespace()
	hpas, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, hpa := range hpas.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(&hpa)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(context.TODO(), hpa.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) getHPAConfig(componentName string, minReplicas *int32, maxReplicas int32, cpuTarget *int32, memoryTarget *int32) *autoscalingv2.HorizontalPodAutoscaler {
	appName := deploy.radixDeployment.Spec.AppName
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}

	metrics := getHpaMetrics(cpuTarget, memoryTarget)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       componentName,
				APIVersion: "apps/v1",
			},
		},
	}

	return hpa
}

func getHpaMetrics(cpuTarget *int32, memoryTarget *int32) []autoscalingv2.MetricSpec {
	var metrics []autoscalingv2.MetricSpec
	if cpuTarget != nil {
		metrics = []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: cpuTarget,
					},
				},
			},
		}
	}

	if memoryTarget != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: memoryTarget,
				},
			},
		})
	}
	return metrics
}

func (deploy *Deployment) deleteHPAIfExists(componentName string) error {
	namespace := deploy.radixDeployment.GetNamespace()
	_, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get hpa: %v", err)
	}
	err = deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(context.TODO(), componentName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete existing hpa: %v", err)
	}

	return nil
}
