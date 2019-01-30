package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrantAppAdminAccessToRuntimeSecrets Grants access to runtime secrets in environment namespace
func (deploy *Deployment) GrantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent) error {
	if component.Secrets == nil || len(component.Secrets) <= 0 {
		return nil
	}

	role := roleAppAdminSecrets(registration, component)

	err := deploy.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminSecrets(registration, role)

	return deploy.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func rolebindingAppAdminSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	subjects := kube.GetRoleBindingGroups(registration.Spec.AdGroups)
	roleName := role.ObjectMeta.Name

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: registration.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: subjects,
	}

	return rolebinding
}
