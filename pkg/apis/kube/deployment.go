package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
)

// ApplyDeployment Create or update deployment in provided namespace
func (kubeutil *Kube) ApplyDeployment(namespace string, currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment) error {
	if currentDeployment == nil {
		createdDeployment, err := kubeutil.CreateDeployment(namespace, desiredDeployment)
		if err != nil {
			return fmt.Errorf("failed to create Deployment object: %v", err)
		}
		log.Debug().Msgf("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
		return nil
	}

	patchBytes, err := getDeploymentPatch(currentDeployment, desiredDeployment)
	if err != nil {
		return err
	}
	if IsEmptyPatch(patchBytes) {
		log.Debug().Msgf("No need to patch deployment: %s ", currentDeployment.GetName())
		return nil
	}
	log.Debug().Msgf("Patch: %s", string(patchBytes))
	patchedDeployment, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).Patch(context.Background(), currentDeployment.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch deployment object: %v", err)
	}
	log.Debug().Msgf("Patched deployment: %s in namespace %s", patchedDeployment.Name, namespace)
	return nil
}

func getDeploymentPatch(currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment) ([]byte, error) {
	currentDeploymentJSON, err := deserializeDeployment(currentDeployment)
	if err != nil {
		return nil, err
	}
	desiredDeploymentJSON, err := deserializeDeployment(desiredDeployment)
	if err != nil {
		return nil, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentDeploymentJSON, desiredDeploymentJSON, appsv1.Deployment{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge patch deployment objects: %v", err)
	}
	return patchBytes, nil
}

func deserializeDeployment(deployment *appsv1.Deployment) ([]byte, error) {
	deployment = deployment.DeepCopy()
	deployment.ObjectMeta.ManagedFields = nil
	delete(deployment.ObjectMeta.Annotations, "deployment.kubernetes.io/revision")
	currentDeploymentJSON, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old deployment object: %v", err)
	}
	return currentDeploymentJSON, nil
}

// CreateDeployment Created deployment
func (kubeutil *Kube) CreateDeployment(namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return kubeutil.KubeClient().AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
}

// DeleteDeployment Delete deployment
func (kubeutil *Kube) DeleteDeployment(namespace, name string) error {
	propagationPolicy := metav1.DeletePropagationBackground
	return kubeutil.KubeClient().AppsV1().Deployments(namespace).Delete(context.Background(),
		name,
		metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
}

// ListDeployments List deployments
func (kubeutil *Kube) ListDeployments(namespace string) ([]*appsv1.Deployment, error) {
	return kubeutil.ListDeploymentsWithSelector(namespace, "")
}

// ListDeploymentsWithSelector List deployments with selector
func (kubeutil *Kube) ListDeploymentsWithSelector(namespace, labelSelectorString string) ([]*appsv1.Deployment, error) {
	var deployments []*appsv1.Deployment

	if kubeutil.DeploymentLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		deployments, err = kubeutil.DeploymentLister.Deployments(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{LabelSelector: labelSelectorString}
		list, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).List(context.TODO(), listOptions)
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

// RadixDeploymentWatcher Watcher to wait for namespace to be created
type RadixDeploymentWatcher interface {
	WaitForActive(namespace, deploymentName string) error
}

// RadixDeploymentWatcherImpl Implementation of watcher
type RadixDeploymentWatcherImpl struct {
	radixClient radixclient.Interface
	waitTimeout time.Duration
}

// NewRadixDeploymentWatcherImpl Constructor
func NewRadixDeploymentWatcherImpl(radixClient radixclient.Interface, waitTimeout time.Duration) RadixDeploymentWatcherImpl {
	return RadixDeploymentWatcherImpl{
		radixClient,
		waitTimeout,
	}
}

// WaitForActive Waits for the radix deployment gets active
func (watcher RadixDeploymentWatcherImpl) WaitForActive(namespace, deploymentName string) error {
	log.Info().Msgf("Waiting while Radix deployment %s in namespace %s gets active", deploymentName, namespace)
	if err := watcher.waitFor(func(context.Context) (bool, error) {
		rd, err := watcher.radixClient.RadixV1().RadixDeployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return rd != nil && !rd.Status.ActiveFrom.IsZero(), nil
	}); err != nil {
		return err
	}

	log.Info().Msgf("Radix deployment %s in namespace %s is active", deploymentName, namespace)
	return nil

}

func (watcher RadixDeploymentWatcherImpl) waitFor(condition wait.ConditionWithContextFunc) error {
	timoutContext, cancel := context.WithTimeout(context.Background(), watcher.waitTimeout)
	defer cancel()
	return wait.PollUntilContextCancel(timoutContext, time.Second, true, condition)
}
