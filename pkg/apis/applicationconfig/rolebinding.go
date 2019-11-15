package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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

	roleBinding := kube.GetRolebindingToClusterRole(app.config.Name, defaults.AppAdminEnvironmentRoleName, adGroups)
	return app.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func rolebindingAppAdminToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	adGroups, _ := application.GetAdGroups(registration)
	roleName := role.ObjectMeta.Name

	return kube.GetRolebindingToRoleWithLabels(roleName, adGroups, role.Labels)
}

func rolebindingPipelineToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	roleName := role.ObjectMeta.Name

	return kube.GetRolebindingToRoleForServiceAccountWithLabels(roleName, defaults.PipelineRoleName, role.Namespace, role.Labels)
}

func garbageCollectAppAdminRoleBindingToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	return garbageCollectRoleBindingToBuildSecrets(kubeclient, namespace, getAppAdminRoleNameToBuildSecrets(name))
}

func garbageCollectPipelineRoleBindingToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	return garbageCollectRoleBindingToBuildSecrets(kubeclient, namespace, getPipelineRoleNameToBuildSecrets(name))
}

func garbageCollectRoleBindingToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
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
