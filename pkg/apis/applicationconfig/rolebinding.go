package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// grantAppAdminAccessToNs Grant access to environment namespace
func (app *ApplicationConfig) grantAppAdminAccessToNs(namespace string) error {
	registration := app.registration

	adGroups, err := application.GetAdGroups(registration)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)
	clusterRoleName := "radix-app-admin-envs"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
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

func (app *ApplicationConfig) grantAccessToPrivateImageHubSecret() error {
	registration := app.registration
	namespace := utils.GetAppNamespace(registration.Name)
	roleName := defaults.PrivateImageHubSecretName
	secretName := defaults.PrivateImageHubSecretName

	// create role
	role := kube.CreateManageSecretRole(registration.GetName(), roleName, []string{secretName}, nil)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	adGroups, err := application.GetAdGroups(registration)
	if err != nil {
		return err
	}
	rolebinding := kube.CreateManageSecretRoleBinding(adGroups, role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
