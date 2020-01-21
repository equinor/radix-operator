package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// ApplyPipelineServiceAccount create service account needed by pipeline
func (app Application) applyPipelineServiceAccount() (*corev1.ServiceAccount, error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	return app.kubeutil.ApplyServiceAccount(defaults.PipelineRoleName, namespace)
}

// ApplyConfigToMapServiceAccount create service account needed by config-to-map-runner
func (app Application) applyConfigToMapServiceAccount() (*corev1.ServiceAccount, error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	return app.kubeutil.ApplyServiceAccount(defaults.ConfigToMapRunnerRoleName, namespace)
}

func (app Application) applyMachineUserServiceAccount(granter GranterFunction) (*corev1.ServiceAccount, error) {
	serviceAccount, err := app.kubeutil.ApplyServiceAccount(defaults.GetMachineUserRoleName(app.registration.Name), utils.GetAppNamespace(app.registration.Name))
	if err != nil {
		return nil, err
	}

	err = app.applyPlatformUserRoleToMachineUser(serviceAccount)
	if err != nil {
		return nil, err
	}

	err = granter(app.kubeutil, app.registration, utils.GetAppNamespace(app.registration.Name), serviceAccount)
	if err != nil {
		return nil, err
	}

	return serviceAccount, nil
}

// GrantAppAdminAccessToMachineUserToken Granter function to grant access to service account token
func GrantAppAdminAccessToMachineUserToken(kubeutil *kube.Kube, app *v1.RadixRegistration, namespace string, serviceAccount *corev1.ServiceAccount) error {
	if len(serviceAccount.Secrets) == 0 {
		return fmt.Errorf("Service account %s currently has no secrets associated with it", serviceAccount.Name)
	}

	role := roleAppAdminMachineUserToken(app.Name, serviceAccount.Secrets[0].Name)
	err := kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	adGroups, _ := GetAdGroups(app)
	rolebinding := rolebindingAppAdminToMachineUserToken(app.Name, adGroups, role)
	return kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
