package deployment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/rs/zerolog/log"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const targetCPUUtilizationPercentage int32 = 80

func (deploy *Deployment) createOrUpdateScaleObject(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()
	horizontalScaling := deployComponent.GetHorizontalScaling()

	// Check if scaler config exists
	if horizontalScaling == nil {
		log.Ctx(ctx).Debug().Msgf("Skip creating ScaledObject %s in namespace %s: no HorizontalScaling config exists", componentName, namespace)
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

	scaler := deploy.getScalerConfig(componentName, horizontalScaling.MinReplicas, horizontalScaling.MaxReplicas, cpuTarget, memoryTarget)

	log.Ctx(ctx).Debug().Msgf("Creating ScaledObject object %s in namespace %s", componentName, namespace)
	createdScaler, err := deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Create(ctx, scaler, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		log.Ctx(ctx).Debug().Msgf("ScaledObject %s already exists in namespace %s, updating the object now", componentName, namespace)
		updatedScaler, err := deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Update(ctx, scaler, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ScaledObject: %v", err)
		}
		log.Ctx(ctx).Debug().Msgf("Updated ScaledObject: %s in namespace %s", updatedScaler.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to create ScaledObject: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Created ScaledObject: %s in namespace %s", createdScaler.Name, namespace)
	return nil
}

func (deploy *Deployment) garbageCollectDeprecatedHPAs(ctx context.Context) error {
	namespace := deploy.radixDeployment.GetNamespace()
	hpas, err := deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, hpa := range hpas.Items {
		err = deploy.kubeclient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, hpa.Name, metav1.DeleteOptions{})
		if err != nil && errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectScalersNoLongerInSpec(ctx context.Context) error {
	namespace := deploy.radixDeployment.GetNamespace()
	scalers, err := deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).List(ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, scaler := range scalers.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(&scaler)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Delete(ctx, scaler.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) getScalerConfig(componentName string, minReplicas *int32, maxReplicas int32, cpuTarget *int32, memoryTarget *int32) *v1alpha1.ScaledObject {
	appName := deploy.radixDeployment.Spec.AppName
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(deploy.radixDeployment),
	}

	triggers := getScalingTriggers(cpuTarget, memoryTarget)

	scaler := &v1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: v1alpha1.ScaledObjectSpec{
			MinReplicaCount: minReplicas,
			MaxReplicaCount: pointers.Ptr(maxReplicas),
			PollingInterval: pointers.Ptr[int32](30), // Default
			Advanced:        &v1alpha1.AdvancedConfig{RestoreToOriginalReplicaCount: true},
			ScaleTargetRef: &v1alpha1.ScaleTarget{
				Kind:       "Deployment",
				Name:       componentName,
				APIVersion: "apps/v1",
			},
			Triggers: triggers,
		},
	}

	return scaler
}

func getScalingTriggers(cpuTarget *int32, memoryTarget *int32) []v1alpha1.ScaleTriggers {
	var triggers []v1alpha1.ScaleTriggers
	if cpuTarget != nil {
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Name:       "cpu",
			Type:       "cpu",
			MetricType: autoscalingv2.UtilizationMetricType,
			Metadata: map[string]string{
				"averageUtilization": strconv.Itoa(int(*cpuTarget)),
			},
		})
	}

	if memoryTarget != nil {
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Name:       "memory",
			Type:       "memory",
			MetricType: autoscalingv2.UtilizationMetricType,
			Metadata: map[string]string{
				"averageUtilization": strconv.Itoa(int(*memoryTarget)),
			},
		})
	}
	return triggers
}

func (deploy *Deployment) deleteScaledObjectIfExists(ctx context.Context, componentName string) error {
	namespace := deploy.radixDeployment.GetNamespace()
	_, err := deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Get(ctx, componentName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get hpa: %v", err)
	}
	err = deploy.kedaClient.KedaV1alpha1().ScaledObjects(namespace).Delete(ctx, componentName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete existing hpa: %v", err)
	}

	return nil
}