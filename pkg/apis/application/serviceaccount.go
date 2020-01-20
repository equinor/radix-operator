package application

import (
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

	return serviceAccount, nil
}
