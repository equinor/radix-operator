package application

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
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

func (app Application) applyMachineUserServiceAccount() (*corev1.ServiceAccount, error) {
	serviceAccount, err := app.kubeutil.ApplyServiceAccount(defaults.GetMachineUserRoleName(app.registration.Name), utils.GetAppNamespace(app.registration.Name))
	if err != nil {
		return nil, err
	}

	err = app.applyPlatformUserRoleToMachineUser(serviceAccount)
	if err != nil {
		return nil, err
	}

	if len(serviceAccount.Secrets) == 0 {
		return nil, fmt.Errorf("Service account %s currently has no secrets associated with it", serviceAccount.Name)
	}

	err = app.grantAppAdminAccessToMachineUserToken(utils.GetAppNamespace(app.registration.Name), serviceAccount.Secrets[0].Name)
	if err != nil {
		return nil, err
	}

	return serviceAccount, nil
}

func (app Application) grantAppAdminAccessToMachineUserToken(namespace, secretName string) error {
	role := roleAppAdminMachineUserToken(app.registration.Name, secretName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	adGroups, _ := GetAdGroups(app.registration)
	rolebinding := rolebindingAppAdminToMachineUserToken(app.registration.Name, adGroups, role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
