package deployment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const targetCPUUtilizationPercentage = 80

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

	hpa := deploy.getHPAConfig(componentName, horizontalScaling.MinReplicas, horizontalScaling.MaxReplicas)

	log.Debugf("Creating HorizontalPodAutoscaler object %s in namespace %s", componentName, namespace)
	createdHPA, err := deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(hpa)
	if errors.IsAlreadyExists(err) {
		log.Debugf("HorizontalPodAutoscaler object %s already exists in namespace %s, updating the object now", componentName, namespace)
		updatedHPA, err := deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(hpa)
		if err != nil {
			return fmt.Errorf("Failed to update HorizontalPodAutoscaler object: %v", err)
		}
		log.Debugf("Updated HorizontalPodAutoscaler: %s in namespace %s", updatedHPA.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create HorizontalPodAutoscaler object: %v", err)
	}
	log.Debugf("Created HorizontalPodAutoscaler: %s in namespace %s", createdHPA.Name, namespace)
	return nil
}

func (deploy *Deployment) garbageCollectHPAsNoLongerInSpec() error {
	namespace := deploy.radixDeployment.GetNamespace()
	hpas, err := deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, hpa := range hpas.Items {
		componentName, ok := NewRadixComponentNameFromLabels(&hpa)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(hpa.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) getHPAConfig(componentName string, minReplicas *int32, maxReplicas int32) *autoscalingv1.HorizontalPodAutoscaler {
	appName := deploy.radixDeployment.Spec.AppName
	ownerReference := getOwnerReferenceOfDeployment(deploy.radixDeployment)
	cpuTarget := int32(targetCPUUtilizationPercentage)

	hpa := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MinReplicas:                    minReplicas,
			MaxReplicas:                    maxReplicas,
			TargetCPUUtilizationPercentage: &cpuTarget,
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
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
	_, err := deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(componentName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to get hpa: %v", err)
	}
	err = deploy.kubeclient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(componentName, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete existing hpa: %v", err)
	}

	return nil
}
