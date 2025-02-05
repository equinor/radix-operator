package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

const (
	externalDnsAdminRoleName  = "radix-app-externaldns-adm"
	externalDnsReaderRoleName = "radix-app-externaldns-reader"
)

func getComponentSecretRbaclabels(appName, componentName string) kubelabels.Set {
	return labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(componentName))
}

func (deploy *Deployment) grantAccessToExternalDnsSecrets(ctx context.Context, secretNames []string) error {
	if len(secretNames) == 0 {
		for _, roleName := range []string{externalDnsAdminRoleName, externalDnsReaderRoleName} {
			err := deploy.kubeclient.RbacV1().Roles(deploy.radixDeployment.Namespace).Delete(ctx, roleName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete external dns secret role: %w", err)
			}
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.Namespace).Delete(ctx, roleName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete external dns secret rolebinding: %w", err)
			}
		}
		return nil
	}

	if err := deploy.grantAdminAccessToSecrets(ctx, externalDnsAdminRoleName, secretNames, nil); err != nil {
		return fmt.Errorf("failed to grant admin access to external dns secrets: %w", err)
	}

	if err := deploy.grantReaderAccessToSecrets(ctx, externalDnsReaderRoleName, secretNames, nil); err != nil {
		return fmt.Errorf("failed to grant reader access to external dns secrets: %w", err)
	}

	return nil
}

func (deploy *Deployment) grantAccessToComponentRuntimeSecrets(ctx context.Context, component radixv1.RadixCommonDeployComponent, secretNames []string) error {
	if len(secretNames) == 0 {
		err := deploy.garbageCollectRoleBindingsNoLongerInSpecForComponent(ctx, component)
		if err != nil {
			return err
		}

		err = deploy.garbageCollectRolesNoLongerInSpecForComponent(ctx, component)
		if err != nil {
			return err
		}

		return nil
	}

	extraLabels := getComponentSecretRbaclabels(deploy.registration.Name, component.GetName())
	adminRoleName := fmt.Sprintf("radix-app-adm-%s", component.GetName())
	readerRoleName := fmt.Sprintf("radix-app-reader-%s", component.GetName())

	if err := deploy.grantAdminAccessToSecrets(ctx, adminRoleName, secretNames, extraLabels); err != nil {
		return err
	}

	return deploy.grantReaderAccessToSecrets(ctx, readerRoleName, secretNames, extraLabels)
}

func (deploy *Deployment) grantAdminAccessToSecrets(ctx context.Context, roleName string, secretNames []string, extraLabels map[string]string) error {
	namespace, registration := deploy.radixDeployment.Namespace, deploy.registration
	subjects, err := utils.GetAppAdminRbacSubjects(registration)
	if err != nil {
		return err
	}
	role := kube.CreateManageSecretRole(registration.Name, roleName, secretNames, extraLabels)
	roleBinding := kube.GetRolebindingToRoleForSubjectsWithLabels(role.ObjectMeta.Name, subjects, role.Labels)

	if err := deploy.kubeutil.ApplyRole(ctx, namespace, role); err != nil {
		return err
	}

	return deploy.kubeutil.ApplyRoleBinding(ctx, namespace, roleBinding)
}

func (deploy *Deployment) grantReaderAccessToSecrets(ctx context.Context, roleName string, secretNames []string, extraLabels map[string]string) error {
	namespace, registration := deploy.radixDeployment.Namespace, deploy.registration

	role := kube.CreateReadSecretRole(registration.Name, roleName, secretNames, extraLabels)
	subjects := utils.GetAppReaderRbacSubjects(registration)
	roleBinding := kube.GetRolebindingToRoleForSubjectsWithLabels(role.ObjectMeta.Name, subjects, role.Labels)

	if err := deploy.kubeutil.ApplyRole(ctx, namespace, role); err != nil {
		return err
	}

	return deploy.kubeutil.ApplyRoleBinding(ctx, namespace, roleBinding)
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	labelSelector := getComponentSecretRbaclabels(deploy.registration.Name, component.GetName()).String()
	roleBindings, err := deploy.kubeutil.ListRoleBindingsWithSelector(ctx, deploy.radixDeployment.GetNamespace(), labelSelector)

	if err != nil {
		return err
	}

	if len(roleBindings) > 0 {
		for _, rb := range roleBindings {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(ctx, rb.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpec(ctx context.Context) error {
	roleBindings, err := deploy.kubeutil.ListRoleBindings(ctx, deploy.radixDeployment.GetNamespace())
	if err != nil {
		return nil
	}

	for _, roleBinding := range roleBindings {
		componentName, ok := RadixComponentNameFromComponentLabel(roleBinding)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(ctx, roleBinding.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
