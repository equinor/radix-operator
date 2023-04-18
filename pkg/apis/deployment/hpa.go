package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const targetCPUUtilizationPercentage int32 = 80
const memoryUtilizationPercentage int32 = 80

func (deploy *Deployment) createOrUpdateHPA(deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()
	replicas := deployComponent.GetReplicas()
	horizontalScaling := deployComponent.GetHorizontalScaling()

	// Check if hpa config exists
	if horizontalScaling == nil {
		log.Debugf("Skip creating HorizontalPodAutoscaler %s in namespace %s: no HorizontalScaling config exists", componentName, namespace)
		return nil
	}

	// Check if replicas == 0
	if replicas != nil && *replicas == 0 {
		log.Debugf("Skip creating HorizontalPodAutoscaler %s in namespace %s: replicas is 0", componentName, namespace)
		return nil
	}

	cpuTarget := targetCPUUtilizationPercentage

	if horizontalScaling.RadixHorizontalScalingResources != nil {
		if horizontalScaling.RadixHorizontalScalingResources.Cpu != nil {
			if horizontalScaling.RadixHorizontalScalingResources.Cpu.AverageUtilization != nil {
				cpuTarget = *horizontalScaling.RadixHorizontalScalingResources.Cpu.AverageUtilization
			}
		}
	}

	memoryTarget := memoryUtilizationPercentage

	if horizontalScaling.RadixHorizontalScalingResources != nil {
		if horizontalScaling.RadixHorizontalScalingResources.Memory != nil {
			if horizontalScaling.RadixHorizontalScalingResources.Memory.AverageUtilization != nil {
				memoryTarget = *horizontalScaling.RadixHorizontalScalingResources.Memory.AverageUtilization
			}
		}
	}

	hpa := deploy.getHPAConfig(componentName, horizontalScaling.MinReplicas, horizontalScaling.MaxReplicas, cpuTarget, memoryTarget)

	log.Debugf("Creating HorizontalPodAutoscaler object %s in namespace %s", componentName, namespace)
	createdHPA, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		log.Debugf("HorizontalPodAutoscaler object %s already exists in namespace %s, updating the object now", componentName, namespace)
		updatedHPA, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update HorizontalPodAutoscaler object: %v", err)
		}
		log.Debugf("Updated HorizontalPodAutoscaler: %s in namespace %s", updatedHPA.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to create HorizontalPodAutoscaler object: %v", err)
	}
	log.Debugf("Created HorizontalPodAutoscaler: %s in namespace %s", createdHPA.Name, namespace)
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

func (deploy *Deployment) getHPAConfig(componentName string, minReplicas *int32, maxReplicas int32, cpuTarget int32, memoryTarget int32) *autoscalingv2.HorizontalPodAutoscaler {
	appName := deploy.radixDeployment.Spec.AppName
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}

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
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &cpuTarget,
						},
					},
				},
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &memoryTarget,
						},
					},
				},
			},

			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       componentName,
				APIVersion: "apps/v1",
			},
		},
	}

	return hpa
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
