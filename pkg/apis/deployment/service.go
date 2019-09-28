package deployment

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func (deploy *Deployment) createService(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	service := getServiceConfig(deployComponent.Name, deploy.radixDeployment, deployComponent.Ports)

	oldService, err := deploy.getService(deployComponent.Name)
	if err != nil && errors.IsNotFound(err) {
		log.Infof("#########YALLA##########Creating Service object %s in namespace %s", deployComponent.Name, namespace)
		createdService, err := deploy.kubeclient.CoreV1().Services(namespace).Create(service)

		if err != nil {
			return fmt.Errorf("Failed to create Service object: %v", err)
		}

		log.Infof("#########YALLA##########Created Service: %s in namespace %s", createdService.Name, namespace)
		return nil
	}

	log.Infof("Service object %s already exists in namespace %s, updating the object now", deployComponent.Name, namespace)
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

	if !isEmptyPatch(patchBytes) {
		patchedService, err := deploy.kubeclient.CoreV1().Services(namespace).Patch(deployComponent.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Service object: %v", err)
		}
		log.Infof("#########YALLA##########Patched Service: %s in namespace %s", patchedService.Name, namespace)
	} else {
		log.Infof("#########YALLA##########No need to patch service: %s ", deployComponent.Name)
	}

	return nil
}

func (deploy *Deployment) garbageCollectServicesNoLongerInSpec() error {
	services, err := deploy.listServices()
	if err != nil {
		return err
	}

	for _, exisitingComponent := range services {
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

func (deploy *Deployment) listServices() ([]*corev1.Service, error) {
	var services []*corev1.Service
	var err error

	if deploy.serviceLister != nil {
		services, err = deploy.serviceLister.Services(deploy.radixDeployment.GetNamespace()).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		services = slice.PointersOf(list.Items).([]*corev1.Service)
	}

	return services, nil
}

func (deploy *Deployment) getService(name string) (*corev1.Service, error) {
	var service *corev1.Service
	var err error

	if deploy.serviceLister != nil {
		service, err = deploy.serviceLister.Services(deploy.radixDeployment.GetNamespace()).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		service, err = deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return service, nil
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

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
