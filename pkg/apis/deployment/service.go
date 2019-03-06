package deployment

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func (deploy *Deployment) createService(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	service := getServiceConfig(deployComponent.Name, deploy.radixDeployment, deployComponent.Ports)
	log.Debugf("Creating Service object %s in namespace %s", deployComponent.Name, namespace)
	createdService, err := deploy.kubeclient.CoreV1().Services(namespace).Create(service)
	if errors.IsAlreadyExists(err) {
		log.Debugf("Service object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
		oldService, err := deploy.kubeclient.CoreV1().Services(namespace).Get(deployComponent.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old Service object: %v", err)
		}
		newService := oldService.DeepCopy()
		ports := buildServicePorts(deployComponent.Ports)

		newService.Spec.Ports = ports
		newService.ObjectMeta.OwnerReferences = service.ObjectMeta.OwnerReferences

		oldServiceJSON, err := json.Marshal(oldService)
		if err != nil {
			return fmt.Errorf("Failed to marshal old Service object: %v", err)
		}

		newServiceJSON, err := json.Marshal(newService)
		if err != nil {
			return fmt.Errorf("Failed to marshal new Service object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldServiceJSON, newServiceJSON, corev1.Service{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch Service objects: %v", err)
		}

		patchedService, err := deploy.kubeclient.CoreV1().Services(namespace).Patch(deployComponent.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Service object: %v", err)
		}
		log.Debugf("Patched Service: %s in namespace %s", patchedService.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create Service object: %v", err)
	}
	log.Debugf("Created Service: %s in namespace %s", createdService.Name, namespace)
	return nil
}

func (deploy *Deployment) garbageCollectServicesNoLongerInSpec() error {
	services, err := deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, exisitingComponent := range services.Items {
		garbageCollect := true
		exisitingComponentName := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getServiceConfig(componentName string, radixDeployment *v1.RadixDeployment, componentPorts []v1.ComponentPort) *corev1.Service {
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":               radixDeployment.Spec.AppName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       radixDeployment.Spec.AppName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
	}

	ports := buildServicePorts(componentPorts)
	service.Spec.Ports = ports

	return service
}

func buildServicePorts(componentPorts []v1.ComponentPort) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Name: v.Name,
			Port: int32(v.Port),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
