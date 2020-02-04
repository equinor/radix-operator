package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	auth "k8s.io/api/rbac/v1"
)

// grantAppAdminAccessToNs Grant access to environment namespace
func (app *ApplicationConfig) grantAppAdminAccessToNs(namespace string) error {
	registration := app.registration

	adGroups, err := application.GetAdGroups(registration)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	if app.registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(app.config.Name),
			Namespace: utils.GetAppNamespace(app.registration.Name),
		})
	}

	roleBinding := kube.GetRolebindingToClusterRoleForSubjects(app.config.Name, defaults.AppAdminEnvironmentRoleName, subjects)
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

	rolebinding := kube.GetRolebindingToRoleWithLabels(roleName, adGroups, role.Labels)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
