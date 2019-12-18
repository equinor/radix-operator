package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyService Will create or update service in provided namespace
func (k *Kube) ApplyService(namespace string, service *corev1.Service) error {
	oldService, err := k.getService(namespace, service.GetName())
	if err != nil && errors.IsNotFound(err) {
		_, err := k.kubeClient.CoreV1().Services(namespace).Create(service)

		if err != nil {
			return fmt.Errorf("Failed to create service object: %v", err)
		}

		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get service object: %v", err)
	}

	log.Debugf("Service object %s already exists in namespace %s, updating the object now", service.GetName(), namespace)
	newService := oldService.DeepCopy()
	newService.Spec.Ports = service.Spec.Ports
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
		patchedService, err := k.kubeClient.CoreV1().Services(namespace).Patch(service.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch Service object: %v", err)
		}
		log.Debugf("Patched Service: %s in namespace %s", patchedService.Name, namespace)
	} else {
		log.Debugf("No need to patch service: %s ", service.GetName())
	}

	return nil
}

// ListServices Lists services from cache or from cluster
func (k *Kube) ListServices(namespace string) ([]*corev1.Service, error) {
	var services []*corev1.Service
	var err error

	if k.ServiceLister != nil {
		services, err = k.ServiceLister.Services(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := k.kubeClient.CoreV1().Services(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		services = slice.PointersOf(list.Items).([]*corev1.Service)
	}

	return services, nil
}

// GetService Get service from cache or from cluster
func (k *Kube) getService(namespace, name string) (*corev1.Service, error) {
	var service *corev1.Service
	var err error

	if k.ServiceLister != nil {
		service, err = k.ServiceLister.Services(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		service, err = k.kubeClient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}
