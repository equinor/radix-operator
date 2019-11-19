package applicationconfig

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (app *ApplicationConfig) grantAccessToBuildSecrets(namespace string) error {
	err := app.grantPipelineAccessToBuildSecrets(namespace)
	if err != nil {
		return err
	}

	err = app.grantAppAdminAccessToBuildSecrets(namespace)
	if err != nil {
		return err
	}

	return nil
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

func roleAppAdminBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	return kube.CreateManageSecretRole(registration.Name, getAppAdminRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func rolePipelineBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	return kube.CreateManageSecretRole(registration.Name, getPipelineRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func garbageCollectAppAdminRoleToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	return garbageCollectRoleToBuildSecrets(kubeclient, namespace, getAppAdminRoleNameToBuildSecrets(name))
}

func garbageCollectPipelineRoleToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	return garbageCollectRoleToBuildSecrets(kubeclient, namespace, getPipelineRoleNameToBuildSecrets(name))
}

func garbageCollectRoleToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	role, err := kubeclient.RbacV1().Roles(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !errors.IsNotFound(err) && role != nil {
		err := kubeclient.RbacV1().Roles(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func getAppAdminRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppAdminRoleName, buildSecretName)
}

func getPipelineRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", "pipeline", buildSecretName)
}
