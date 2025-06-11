package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyDeployment Create or update deployment in provided namespace
func (kubeutil *Kube) ApplyDeployment(ctx context.Context, namespace string, currentDeployment *appsv1.Deployment, desiredDeployment *appsv1.Deployment, apply447hack bool) error {
	if currentDeployment == nil {
		createdDeployment, err := kubeutil.CreateDeployment(ctx, namespace, desiredDeployment)
		if err != nil {
			return fmt.Errorf("failed to create Deployment object: %v", err)
		}
		log.Ctx(ctx).Debug().Msgf("Created Deployment: %s in namespace %s", createdDeployment.Name, namespace)
		return nil
	}

	patchBytes, err := getDeploymentPatch(currentDeployment, desiredDeployment)
	if err != nil {
		return err
	}
	if IsEmptyPatch(patchBytes) {
		log.Ctx(ctx).Debug().Msgf("No need to patch deployment: %s ", currentDeployment.GetName())
		return nil
	}

	// Ignore the 447 hack if its a job scheduler or oauth2proxy etc. Only apply the hack for components
	// (we want to avoid restarting components when only the Radix App ID is changed)
	if apply447hack {
		if isAppIdTheOnlyChange_akaThe447Hack(*currentDeployment, *desiredDeployment) {
			log.Ctx(ctx).Debug().Msgf("Only RadixAppIDLabel changed for deployment: %s, skipping patch", currentDeployment.GetName())
			return nil
		}
	}

	log.Ctx(ctx).Debug().Msgf("Patch: %s", string(patchBytes))
	patchedDeployment, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).Patch(ctx, currentDeployment.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch deployment object: %v", err)
	}
	log.Ctx(ctx).Debug().Msgf("Patched deployment: %s in namespace %s", patchedDeployment.Name, namespace)
	return nil
}

// isAppIdTheOnlyChange_akaThe447Hack checks if the only difference between the current and desired deployment is the RadixAppIDLabel.
// This is to avoid restarting radix-apps when the Radix App ID is changed. Remove this code and force sync all deployments
// when _most_ apps have redeployed and is using AppID. See https://github.com/equinor/radix/issues/447 and its parent issue for more details
func isAppIdTheOnlyChange_akaThe447Hack(currentDeployment, desiredDeployment appsv1.Deployment) bool {

	currentLabels := maps.Clone(currentDeployment.Spec.Template.ObjectMeta.Labels)
	desiredLabels := maps.Clone(desiredDeployment.Spec.Template.ObjectMeta.Labels)
	currentDeployment.Spec.Template.ObjectMeta.Labels = nil
	desiredDeployment.Spec.Template.ObjectMeta.Labels = nil

	patchBytes, _ := getDeploymentPatch(&currentDeployment, &desiredDeployment)
	specIsEqual := IsEmptyPatch(patchBytes)

	appIDIsDifferent := currentLabels[RadixAppIDLabel] != desiredLabels[RadixAppIDLabel]
	delete(currentLabels, RadixAppIDLabel)
	delete(desiredLabels, RadixAppIDLabel)
	labelsAreEqual := reflect.DeepEqual(currentLabels, desiredLabels)

	return (appIDIsDifferent && specIsEqual && labelsAreEqual)
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
func (kubeutil *Kube) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return kubeutil.KubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

// DeleteDeployment Delete deployment
func (kubeutil *Kube) DeleteDeployment(ctx context.Context, namespace, name string) error {
	propagationPolicy := metav1.DeletePropagationBackground
	return kubeutil.KubeClient().AppsV1().Deployments(namespace).Delete(ctx,
		name,
		metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
}

// ListDeployments List deployments
func (kubeutil *Kube) ListDeployments(ctx context.Context, namespace string) ([]*appsv1.Deployment, error) {
	return kubeutil.ListDeploymentsWithSelector(ctx, namespace, "")
}

// ListDeploymentsWithSelector List deployments with selector
func (kubeutil *Kube) ListDeploymentsWithSelector(ctx context.Context, namespace, labelSelectorString string) ([]*appsv1.Deployment, error) {
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
		list, err := kubeutil.kubeClient.AppsV1().Deployments(namespace).List(ctx, listOptions)
		if err != nil {
			return nil, err
		}

		deployments = slice.PointersOf(list.Items).([]*appsv1.Deployment)
	}

	return deployments, nil
}

func (kubeutil *Kube) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment
	var err error

	if kubeutil.DeploymentLister != nil {
		deployment, err = kubeutil.DeploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		deployment, err = kubeutil.kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}
