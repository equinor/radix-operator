package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixannotations "github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func (deploy *Deployment) createOrUpdateServiceAccount(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	if isServiceAccountForComponentPreinstalled(deploy.radixDeployment, component) {
		return nil
	}

	if !componentRequiresServiceAccount(component) {
		return nil
	}

	sa := buildServiceAccountForComponent(deploy.radixDeployment.Namespace, component)

	if err := verifyServiceAccountForComponentCanBeApplied(ctx, deploy.kubeutil, sa, component); err != nil {
		return err
	}

	_, err := deploy.kubeutil.ApplyServiceAccount(ctx, sa)
	return err
}

func createOrUpdateOAuthProxyServiceAccount(ctx context.Context, kubeUtil *kube.Kube, radixDeployment *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) error {
	if isServiceAccountForComponentPreinstalled(radixDeployment, component) {
		return nil
	}

	if !component.GetAuthentication().GetOAuth2().GetUseAzureIdentity() {
		return nil
	}

	sa := buildServiceAccountForOAuthProxyComponent(radixDeployment.Namespace, component)

	if err := verifyServiceAccountForOAuthProxyComponentCanBeApplied(ctx, kubeUtil, sa, component); err != nil {
		return err
	}

	_, err := kubeUtil.ApplyServiceAccount(ctx, sa)
	return err
}

func verifyServiceAccountForComponentCanBeApplied(ctx context.Context, kubeUtil *kube.Kube, sa *corev1.ServiceAccount, component radixv1.RadixCommonDeployComponent) error {
	existingSa, err := kubeUtil.GetServiceAccount(ctx, sa.GetNamespace(), sa.GetName())
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

func verifyServiceAccountForOAuthProxyComponentCanBeApplied(ctx context.Context, kubeUtil *kube.Kube, sa *corev1.ServiceAccount, component radixv1.RadixCommonDeployComponent) error {
	existingSa, err := kubeUtil.GetServiceAccount(ctx, sa.GetNamespace(), sa.GetName())
	if err != nil {
		// If service account does not already exist we return nil to indicate that it can be applied
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !isServiceAccountForOAuthProxyComponent(existingSa, component) {
		return fmt.Errorf("existing service account does not belong to aux OAuth proxy of the component %s", component.GetName())
	}

	return nil
}

func buildServiceAccountForComponent(namespace string, component radixv1.RadixCommonDeployComponent) *corev1.ServiceAccount {
	labels := getComponentServiceAccountLabels(component)
	name := utils.GetComponentServiceAccountName(component.GetName())
	return buildServiceAccount(namespace, name, labels, component)
}

func buildServiceAccountForOAuthProxyComponent(namespace string, component radixv1.RadixCommonDeployComponent) *corev1.ServiceAccount {
	labels := getOAuthProxyComponentServiceAccountLabels(component)
	name := utils.GetOAuthProxyServiceAccountName(component.GetName())
	return buildServiceAccount(namespace, name, labels, component)
}

func buildServiceAccount(namespace, name string, labels kubelabels.Set, component radixv1.RadixCommonDeployComponent) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: getComponentServiceAccountAnnotations(component),
		},
	}
}

func (deploy *Deployment) garbageCollectServiceAccountNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	if isServiceAccountForComponentPreinstalled(deploy.radixDeployment, component) {
		return nil
	}

	if componentRequiresServiceAccount(component) {
		return nil
	}

	serviceAccountList, err := deploy.kubeutil.ListServiceAccountsWithSelector(ctx, deploy.radixDeployment.Namespace, getComponentServiceAccountIdentifier(component.GetName()).AsSelector().String())
	if err != nil {
		return err
	}

	for _, sa := range serviceAccountList {
		if err := deploy.kubeutil.DeleteServiceAccount(ctx, sa.Namespace, sa.Name); err != nil {
			return err
		}
	}

	return nil
}

func garbageCollectServiceAccountNoLongerInSpecForOAuthProxyComponent(ctx context.Context, kubeUtil *kube.Kube, radixDeployment *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) error {
	if isServiceAccountForComponentPreinstalled(radixDeployment, component) {
		return nil
	}

	if component.GetAuthentication().GetOAuth2().GetUseAzureIdentity() {
		return nil
	}

	return deleteOAuthProxyServiceAccounts(ctx, kubeUtil, radixDeployment.Namespace, component)
}

func deleteOAuthProxyServiceAccounts(ctx context.Context, kubeUtil *kube.Kube, namespace string, component radixv1.RadixCommonDeployComponent) error {
	selector := radixlabels.ForOAuthProxyComponentServiceAccount(component).AsSelector().String()
	serviceAccountList, err := kubeUtil.ListServiceAccountsWithSelector(ctx, namespace, selector)
	if err != nil {
		return err
	}

	for _, sa := range serviceAccountList {
		if err := kubeUtil.DeleteServiceAccount(ctx, namespace, sa.Name); err != nil {
			return err
		}
		log.Ctx(ctx).Info().Msgf("Deleted serviceAccount: %s in namespace %s", sa.Name, namespace)
	}

	return nil
}

func (deploy *Deployment) garbageCollectServiceAccountNoLongerInSpec(ctx context.Context) error {
	serviceAccounts, err := deploy.kubeutil.ListServiceAccounts(ctx, deploy.radixDeployment.GetNamespace())
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

			if err := deploy.kubeclient.CoreV1().ServiceAccounts(deploy.radixDeployment.GetNamespace()).Delete(ctx, serviceAccount.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// Is this a component/deployment where the service account is handled elsewhere,
// e.g. radix-api and radix-github-webhook
func isServiceAccountForComponentPreinstalled(radixDeployment *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent) bool {
	isComponent := component.GetType() == radixv1.RadixComponentTypeComponent
	return isComponent && (isRadixAPI(radixDeployment) || isRadixWebHook(radixDeployment))
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

func getOAuthProxyComponentServiceAccountLabels(component radixv1.RadixCommonDeployComponent) kubelabels.Set {
	return radixlabels.Merge(
		radixlabels.ForOAuthProxyComponentServiceAccount(component),
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

func isServiceAccountForComponent(sa *corev1.ServiceAccount, componentName string) bool {
	return getComponentServiceAccountIdentifier(componentName).AsSelector().Matches(kubelabels.Set(sa.Labels))
}

func isServiceAccountForOAuthProxyComponent(sa *corev1.ServiceAccount, component radixv1.RadixCommonDeployComponent) bool {
	return radixlabels.ForOAuthProxyComponentServiceAccount(component).AsSelector().Matches(kubelabels.Set(sa.Labels))
}
