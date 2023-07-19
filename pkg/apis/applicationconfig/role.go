package applicationconfig

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/sirupsen/logrus"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (app *ApplicationConfig) grantAccessToBuildSecrets(namespace string) error {
	err := app.grantPipelineAccessToBuildSecrets(namespace)
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

func (app *ApplicationConfig) grantPipelineAccessToBuildSecrets(namespace string) error {
	role := rolePipelineBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingPipelineToBuildSecrets(app.GetRadixRegistration(), role)
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
		getPipelineRoleNameToBuildSecrets(defaults.BuildSecretsName),
		getAppReaderRoleNameToBuildSecrets(defaults.BuildSecretsName),
		getAppAdminRoleNameToBuildSecrets(defaults.BuildSecretsName),
	} {
		err := app.garbageCollectAccessToBuildSecretsForRole(appNamespace, roleName)
		if err != nil {
			logrus.Warnf("Failed to perform garbage collection of access to build secret: %v", err)
			return err
		}
	}
	return nil
}

func roleAppAdminBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	return kube.CreateManageSecretRole(registration.Name, getAppAdminRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func roleAppReaderBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	return kube.CreateReadSecretRole(registration.Name, getAppReaderRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func rolePipelineBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	return kube.CreateManageSecretRole(registration.Name, getPipelineRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func getAppAdminRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppAdminRoleName, buildSecretName)
}

func getAppReaderRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppReaderRoleName, buildSecretName)
}

func getPipelineRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", "pipeline", buildSecretName)
}
