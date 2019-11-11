package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// grantAppAdminAccessToNs Grant access to environment namespace
func (app *ApplicationConfig) grantAppAdminAccessToNs(namespace string) error {
	registration := app.registration

	adGroups, err := application.GetAdGroups(registration)
	if err != nil {
		return err
	}

	clusterRoleName := "radix-app-admin-envs"
	roleBinding := kube.GetRolebindingToClusterRole(app.config.Name, clusterRoleName, adGroups)
	return app.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func rolebindingPipelineToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	adGroups, _ := application.GetAdGroups(registration)
	roleName := role.ObjectMeta.Name

	return kube.GetRolebindingToRoleWithLabels(roleName, adGroups, role.Labels)
}

func garbageCollectAppAdminRoleBindingToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	roleBinding, err := kubeclient.RbacV1().RoleBindings(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !errors.IsNotFound(err) && roleBinding != nil {
		err := kubeclient.RbacV1().RoleBindings(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
