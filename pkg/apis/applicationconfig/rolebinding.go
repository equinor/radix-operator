package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// grantAppAdminAccessToNs Grant access to environment namespace
func (app *ApplicationConfig) grantAppAdminAccessToNs(namespace string) error {
	registration := app.registration
	subjects := kube.GetRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin-envs"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":         app.config.Name, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: app.config.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: subjects,
	}

	return app.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}
