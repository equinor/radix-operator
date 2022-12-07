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

	if !isServiceAccountForComponent(existingSa, component.GetName()) {
		return fmt.Errorf("existing service account does not belong to component %s", component.GetName())
	}

	return nil
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

	serviceAccountList, err := deploy.kubeutil.ListServiceAccountsWithSelector(deploy.radixDeployment.Namespace, getComponentServiceAccountIdentifier(component.GetName()).AsSelector().String())
	if err != nil {
		return err
	}

	for _, sa := range serviceAccountList {
		if err := deploy.kubeutil.DeleteServiceAccount(sa.Namespace, sa.Name); err != nil {
			return err
		}
	}

	return nil
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
			// Skip garbage collect if service account is not labelled as a component/job service account
			if !isServiceAccountForComponent(serviceAccount, string(componentName)) {
				continue
			}

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
		getComponentServiceAccountIdentifier(component.GetName()),
		radixlabels.ForServiceAccountWithRadixIdentity(component.GetIdentity()),
	)
}

func getComponentServiceAccountIdentifier(componentName string) kubelabels.Set {
	return radixlabels.Merge(
		radixlabels.ForComponentName(componentName),
		radixlabels.ForServiceAccountIsForComponent(),
	)
}

func getComponentServiceAccountAnnotations(component radixv1.RadixCommonDeployComponent) map[string]string {
	return radixannotations.ForServiceAccountWithRadixIdentity(component.GetIdentity())
}

// isServiceAccountForComponent checks if service account has labels matching labels returned by getComponentServiceAccountIdentifier
func isServiceAccountForComponent(sa *corev1.ServiceAccount, componentName string) bool {
	return getComponentServiceAccountIdentifier(componentName).AsSelector().Matches(kubelabels.Set(sa.Labels))
}
