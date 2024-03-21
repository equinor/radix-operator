package applicationconfig

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (app *ApplicationConfig) grantAccessToBuildSecrets(namespace string) error {
	err := app.grantPipelineAccessToSecret(namespace, defaults.BuildSecretsName)
	if err != nil {
		return err
	}

	err = app.grantAppReaderAccessToBuildSecrets(namespace)
	if err != nil {
		return err
	}

	err = app.grantAppAdminAccessToBuildSecrets(namespace)
	if err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) grantAppReaderAccessToBuildSecrets(namespace string) error {
	role := roleAppReaderBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppReaderToBuildSecrets(app.GetRadixRegistration(), role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)

}

func (app *ApplicationConfig) grantAppAdminAccessToBuildSecrets(namespace string) error {
	role := roleAppAdminBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminToBuildSecrets(app.GetRadixRegistration(), role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (app *ApplicationConfig) grantPipelineAccessToSecret(namespace, secretName string) error {
	role := rolePipelineSecret(app.GetRadixRegistration(), secretName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingPipelineToRole(role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (app *ApplicationConfig) garbageCollectAccessToBuildSecretsForRole(namespace string, roleName string) error {
	// Delete role
	_, err := app.kubeutil.GetRole(namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = app.kubeutil.DeleteRole(namespace, roleName)
		if err != nil {
			return err
		}
	}

	// Delete roleBinding
	_, err = app.kubeutil.GetRoleBinding(namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = app.kubeutil.DeleteRoleBinding(namespace, roleName)
		if err != nil {
			return err
		}
	}

	return nil
}
func garbageCollectAccessToBuildSecrets(app *ApplicationConfig) error {
	appNamespace := utils.GetAppNamespace(app.config.Name)
	for _, roleName := range []string{
		getPipelineRoleNameToSecret(defaults.BuildSecretsName),
		getAppReaderRoleNameToBuildSecrets(defaults.BuildSecretsName),
		getAppAdminRoleNameToBuildSecrets(defaults.BuildSecretsName),
	} {
		err := app.garbageCollectAccessToBuildSecretsForRole(appNamespace, roleName)
		if err != nil {
			return err
		}
	}
	return nil
}

func roleAppAdminBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *rbacv1.Role {
	return kube.CreateManageSecretRole(registration.Name, getAppAdminRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func roleAppReaderBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *rbacv1.Role {
	return kube.CreateReadSecretRole(registration.Name, getAppReaderRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func rolePipelineSecret(registration *radixv1.RadixRegistration, secretName string) *rbacv1.Role {
	return kube.CreateReadSecretRole(registration.Name, getPipelineRoleNameToSecret(secretName), []string{secretName}, nil)
}

func getAppAdminRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppAdminRoleName, buildSecretName)
}

func getAppReaderRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppReaderRoleName, buildSecretName)
}

func getPipelineRoleNameToSecret(secretName string) string {
	return fmt.Sprintf("%s-%s", "pipeline", secretName)
}
