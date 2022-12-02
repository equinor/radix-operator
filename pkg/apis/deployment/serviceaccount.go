package deployment

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func (deploy *Deployment) createOrUpdateServiceAccount(component radixv1.RadixCommonDeployComponent) error {
	if deploy.isServiceAccountForComponentPreinstalled(component) {
		return nil
	}

	if !componentRequiresServiceAccount(component) {
		return nil
	}

	sa := deploy.buildServiceAccountForComponent(component)

	if err := deploy.verifyServiceAccountForComponentCanBeApplied(sa, component); err != nil {
		return err
	}

	_, err := deploy.kubeutil.ApplyServiceAccount(sa)
	return err
}

func (deploy *Deployment) verifyServiceAccountForComponentCanBeApplied(sa *corev1.ServiceAccount, component radixv1.RadixCommonDeployComponent) error {
	existingSa, err := deploy.kubeutil.GetServiceAccount(sa.GetNamespace(), sa.GetName())
	if err != nil {
		// If service account does not already exist we return nil to indicate that it can be applied
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// The "default" account always exist, and we can safely update it
	if existingSa.GetName() == "default" {
		return nil
	}

	// Allow new service account to be applied if existing service account has label radix-component matching the component name
	componentNameLabel, _ := RadixComponentNameFromComponentLabel(existingSa)
	if string(componentNameLabel) == component.GetName() {
		return nil
	}

	return fmt.Errorf("existing service account does not belong to component %s", component.GetName())
}

func (deploy *Deployment) buildServiceAccountForComponent(component radixv1.RadixCommonDeployComponent) *corev1.ServiceAccount {
	labels := getComponentServiceAccountLabels(component)
	annotations := getComponentServiceAccountAnnotations(component)

	sa := corev1.ServiceAccount{
		ObjectMeta: v1.ObjectMeta{
			Name:        utils.GetComponentServiceAccountName(component.GetName()),
			Namespace:   deploy.radixDeployment.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	return &sa
}

func (deploy *Deployment) garbageCollectServiceAccountNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	if deploy.isServiceAccountForComponentPreinstalled(component) {
		return nil
	}

	if componentRequiresServiceAccount(component) {
		return nil
	}

	sa, err := deploy.kubeutil.GetServiceAccount(deploy.radixDeployment.Namespace, utils.GetComponentServiceAccountName(component.GetName()))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Fail deletion if service account is not labelled as belonging to the componenet
	if _, ok := RadixComponentNameFromComponentLabel(sa); !ok {
		// Ignore missing labels for "default" server account
		// This should only happen if component is named "default"
		// After initial deletion of correctly labeled "default" service account, Kubernetes
		// will automatically recreate it. A new sync of RD must therefore ignore the "default"
		// service account if it is missing expected labels
		if sa.GetName() == "default" {
			return nil
		}
		return fmt.Errorf("service account %s cannot be deleted because ot does not belong to component %s", sa.GetName(), component.GetName())
	}

	return deploy.kubeutil.DeleteServiceAccount(sa.Namespace, sa.Name)
}

func (deploy *Deployment) garbageCollectServiceAccountNoLongerInSpec() error {
	serviceAccounts, err := deploy.kubeutil.ListServiceAccounts(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, serviceAccount := range serviceAccounts {
		componentName, ok := RadixComponentNameFromComponentLabel(serviceAccount)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			if err := deploy.kubeclient.CoreV1().ServiceAccounts(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), serviceAccount.GetName(), v1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// Is this a component/deployment where the service account is handled elsewhere,
// e.g. radix-api and radix-github-webhook
func (deploy *Deployment) isServiceAccountForComponentPreinstalled(component radixv1.RadixCommonDeployComponent) bool {
	isComponent := component.GetType() == radixv1.RadixComponentTypeComponent
	return isComponent && (isRadixAPI(deploy.radixDeployment) || isRadixWebHook(deploy.radixDeployment))
}

func componentRequiresServiceAccount(component radixv1.RadixCommonDeployComponent) bool {
	identity := component.GetIdentity()
	return identity != nil && identity.Azure != nil
}

func getComponentServiceAccountLabels(component radixv1.RadixCommonDeployComponent) kubelabels.Set {
	return radixlabels.Merge(
		radixlabels.ForComponentName(component.GetName()),
		radixlabels.ForServiceAccountWithRadixIdentity(component.GetIdentity()),
	)
}

func getComponentServiceAccountAnnotations(component radixv1.RadixCommonDeployComponent) map[string]string {
	return radixannotations.ForServiceAccountWithRadixIdentity(component.GetIdentity())
}
