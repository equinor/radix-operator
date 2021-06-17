package deployment

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) grantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component radixv1.RadixCommonDeployComponent, secretNames []string) error {
	if len(secretNames) <= 0 {
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

	role := roleAppAdminSecrets(registration, component, secretNames)
	err := deploy.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminSecrets(registration, role)
	return deploy.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	labelSelector := getLabelSelectorForComponent(component)
	roleBindings, err := deploy.kubeutil.ListRoleBindingsWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)

	if err != nil {
		return err
	}

	if len(roleBindings) > 0 {
		for n := range roleBindings {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), roleBindings[n].Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpec() error {
	roleBindings, err := deploy.kubeutil.ListRoleBindings(deploy.radixDeployment.GetNamespace())

	for _, roleBinding := range roleBindings {
		componentName, ok := NewRadixComponentNameFromLabels(roleBinding)
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

func rolebindingAppAdminSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	adGroups, _ := application.GetAdGroups(registration)
	roleName := role.ObjectMeta.Name

	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	if registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(registration.Name),
			Namespace: utils.GetAppNamespace(registration.Name),
		})
	}

	return kube.GetRolebindingToRoleForSubjectsWithLabels(registration.Name, roleName, subjects, role.Labels)
}
