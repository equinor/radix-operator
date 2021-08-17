package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyDeployment Create or update deployment in provided namespace
func (kubeutil *Kube) ApplyDeployment(namespace string, currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment) error {
	if currentDeployment == nil {
		createdDeployment, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).Create(context.TODO(), desiredDeployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Failed to create Deployment object: %v", err)
		}
		log.Debugf("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
		return nil
	}

	currentDeploymentJSON, err := json.Marshal(currentDeployment)
	if err != nil {
		return fmt.Errorf("Failed to marshal old deployment object: %v", err)
	}

	desiredDeploymentJSON, err := json.Marshal(desiredDeployment)
	if err != nil {
		return fmt.Errorf("Failed to marshal new deployment object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentDeploymentJSON, desiredDeploymentJSON, appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch deployment objects: %v", err)
	}

	if utils.IsEmptyPatch(patchBytes) {
		log.Debugf("No need to patch deployment: %s ", currentDeployment.GetName())
		return nil
	}

	log.Debugf("Patch: %s", string(patchBytes))
	patchedDeployment, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).Patch(context.TODO(), currentDeployment.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("Failed to patch deployment object: %v", err)
	}
	log.Debugf("Patched deployment: %s in namespace %s", patchedDeployment.Name, namespace)
	return nil
}

// ListDeployments List deployments
func (kubeutil *Kube) ListDeployments(namespace string) ([]*appsv1.Deployment, error) {
	var deployments []*appsv1.Deployment
	var err error

	if kubeutil.DeploymentLister != nil {
		deployments, err = kubeutil.DeploymentLister.Deployments(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		deployments = slice.PointersOf(list.Items).([]*appsv1.Deployment)
	}

	return deployments, nil
}

func (kubeutil *Kube) GetDeployment(namespace, name string) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment
	var err error

	if kubeutil.DeploymentLister != nil {
		deployment, err = kubeutil.DeploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		deployment, err = kubeutil.kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}
