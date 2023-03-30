package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	auth "k8s.io/api/rbac/v1"
)

func rolebindingAppAdminToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	adGroups, _ := application.GetAdGroups(registration)

	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	if registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(registration.Name),
			Namespace: utils.GetAppNamespace(registration.Name),
		})
	}

	roleName := role.ObjectMeta.Name

	return kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
}

func rolebindingPipelineToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	roleName := role.ObjectMeta.Name

	return kube.GetRolebindingToRoleForServiceAccountWithLabels(roleName, defaults.PipelineServiceAccountName, role.Namespace, role.Labels)
}

// GrantAppAdminAccessToSecret grants access to a secret for app-admin groups
func GrantAppAdminAccessToSecret(kubeutil *kube.Kube, registration *radixv1.RadixRegistration, roleName string, secretName string) error {
	namespace := utils.GetAppNamespace(registration.Name)

	// create role
	role := kube.CreateManageSecretRole(registration.GetName(), roleName, []string{secretName}, nil)
	err := kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	adGroups, err := application.GetAdGroups(registration)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	if registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(registration.Name),
			Namespace: utils.GetAppNamespace(registration.Name),
		})
	}

	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	return kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
