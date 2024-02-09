package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
)

func getComponentSecretRbaclabels(appName, componentName string) kubelabels.Set {
	return labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(componentName))
}

func (deploy *Deployment) grantAccessToComponentRuntimeSecrets(component radixv1.RadixCommonDeployComponent, secretNames []string) error {
	if len(secretNames) == 0 {
		err := deploy.garbageCollectRoleBindingsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}

		err = deploy.garbageCollectRolesNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}

		return nil
	}

	namespace, registration := deploy.radixDeployment.Namespace, deploy.registration
	extraLabels := getComponentSecretRbaclabels(registration.Name, component.GetName())

	// App admin role and rolebinding
	adminRoleName := fmt.Sprintf("radix-app-adm-%s", component.GetName())
	adminGroups, err := utils.GetAdGroups(registration)
	if err != nil {
		return err
	}
	adminRole := kube.CreateManageSecretRole(registration.Name, adminRoleName, secretNames, extraLabels)
	adminRoleBinding := roleBindingAppSecrets(registration.Name, adminRole, adminGroups)

	// App reader role and rolebinding
	readerRoleName := fmt.Sprintf("radix-app-reader-%s", component.GetName())
	readerRole := kube.CreateReadSecretRole(registration.Name, readerRoleName, secretNames, extraLabels)
	readerRoleBinding := roleBindingAppSecrets(registration.Name, readerRole, registration.Spec.ReaderAdGroups)

	// Apply roles and rolebindings
	for _, role := range []*rbacv1.Role{adminRole, readerRole} {
		if err := deploy.kubeutil.ApplyRole(namespace, role); err != nil {
			return err
		}
	}
	for _, roleBinding := range []*rbacv1.RoleBinding{adminRoleBinding, readerRoleBinding} {
		if err := deploy.kubeutil.ApplyRoleBinding(namespace, roleBinding); err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	labelSelector := getComponentSecretRbaclabels(deploy.registration.Name, component.GetName()).String()
	roleBindings, err := deploy.kubeutil.ListRoleBindingsWithSelector(deploy.radixDeployment.GetNamespace(), labelSelector)

	if err != nil {
		return err
	}

	if len(roleBindings) > 0 {
		for _, rb := range roleBindings {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), rb.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpec() error {
	roleBindings, err := deploy.kubeutil.ListRoleBindings(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return nil
	}

	for _, roleBinding := range roleBindings {
		componentName, ok := RadixComponentNameFromComponentLabel(roleBinding)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func roleBindingAppSecrets(appName string, role *rbacv1.Role, groups []string) *rbacv1.RoleBinding {
	roleName := role.ObjectMeta.Name
	subjects := kube.GetRoleBindingGroups(groups)
	return kube.GetRolebindingToRoleForSubjectsWithLabels(appName, roleName, subjects, role.Labels)
}
