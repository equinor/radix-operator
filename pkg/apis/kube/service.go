package kube

import (
	"context"
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
func (kubeutil *Kube) ApplyService(namespace string, service *corev1.Service) error {
	oldService, err := kubeutil.getService(namespace, service.GetName())
	if err != nil && errors.IsNotFound(err) {
		_, err := kubeutil.kubeClient.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})

		if err != nil {
			return fmt.Errorf("failed to create service object: %v", err)
		}

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get service object: %v", err)
	}

	log.Debugf("Service object %s already exists in namespace %s, updating the object now", service.GetName(), namespace)
	newService := oldService.DeepCopy()
	newService.Spec.Ports = service.Spec.Ports
	newService.ObjectMeta.OwnerReferences = service.ObjectMeta.OwnerReferences
	newService.ObjectMeta.Labels = service.ObjectMeta.Labels
	newService.Spec.Selector = service.Spec.Selector

	oldServiceJSON, err := json.Marshal(oldService)
	if err != nil {
		return fmt.Errorf("failed to marshal old Service object: %v", err)
	}

	newServiceJSON, err := json.Marshal(newService)
	if err != nil {
		return fmt.Errorf("failed to marshal new Service object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldServiceJSON, newServiceJSON, corev1.Service{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch Service objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedService, err := kubeutil.kubeClient.CoreV1().Services(namespace).Patch(context.TODO(), service.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch Service object: %v", err)
		}
		log.Debugf("Patched Service: %s in namespace %s", patchedService.Name, namespace)
	} else {
		log.Debugf("No need to patch service: %s ", service.GetName())
	}

	return nil
}

// ListServices Lists services from cache or from cluster
func (kubeutil *Kube) ListServices(namespace string) ([]*corev1.Service, error) {
	return kubeutil.ListServicesWithSelector(namespace, "")
}

// ListServices Lists services from cache or from cluster
func (kubeutil *Kube) ListServicesWithSelector(namespace, labelSelectorString string) ([]*corev1.Service, error) {
	var services []*corev1.Service

	if kubeutil.ServiceLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		services, err = kubeutil.ServiceLister.Services(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{LabelSelector: labelSelectorString}
		list, err := kubeutil.kubeClient.CoreV1().Services(namespace).List(context.TODO(), listOptions)
		if err != nil {
			return nil, err
		}

		services = slice.PointersOf(list.Items).([]*corev1.Service)
	}

	return services, nil
}

// GetService Get service from cache or from cluster
func (kubeutil *Kube) getService(namespace, name string) (*corev1.Service, error) {
	var service *corev1.Service
	var err error

	if kubeutil.ServiceLister != nil {
		service, err = kubeutil.ServiceLister.Services(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		service, err = kubeutil.kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}
