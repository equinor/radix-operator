package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyPipelineServiceAccount create service account needed by pipeline
func (app Application) applyPipelineServiceAccount() (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.PipelineRoleName,
			Namespace: utils.GetAppNamespace(app.registration.Name),
		},
	}

	return app.kubeutil.ApplyServiceAccount(serviceAccount)
}

// ApplyConfigToMapServiceAccount create service account needed by config-to-map-runner
func (app Application) applyConfigToMapServiceAccount() (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.ConfigToMapRunnerRoleName,
			Namespace: utils.GetAppNamespace(app.registration.Name),
		},
	}

	return app.kubeutil.ApplyServiceAccount(serviceAccount)
}

// applyScanImageServiceAccount create service account needed by scan-image-runner
func (app Application) applyScanImageServiceAccount() (*corev1.ServiceAccount, error) {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.ScanImageRunnerRoleName,
			Namespace: utils.GetAppNamespace(app.registration.Name),
		},
	}

	return app.kubeutil.ApplyServiceAccount(serviceAccount)
}

func (app Application) applyMachineUserServiceAccount(granter GranterFunction) (*corev1.ServiceAccount, error) {
	newServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetMachineUserRoleName(app.registration.Name),
			Namespace: utils.GetAppNamespace(app.registration.Name),
		},
	}

	serviceAccount, err := app.kubeutil.ApplyServiceAccount(newServiceAccount)
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

func (app Application) garbageCollectMachineUserServiceAccount() error {
	err := app.kubeutil.DeleteServiceAccount(utils.GetAppNamespace(app.registration.Name), defaults.GetMachineUserRoleName(app.registration.Name))
	if err != nil {
		return err
	}
	return nil
}

func (app Application) removeMachineUserFromPlatformUserRole() error {
	err := app.kubeutil.DeleteClusterRoleBinding(defaults.GetMachineUserRoleName(app.registration.Name))
	if err != nil {
		return err
	}
	return nil
}

func (app Application) removeAppAdminAccessToMachineUserToken() error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	name := fmt.Sprintf("%s-%s", defaults.GetMachineUserRoleName(app.registration.Name), "token")

	err := app.kubeutil.DeleteRoleBinding(namespace, name)
	if err != nil {
		return err
	}

	err = app.kubeutil.DeleteRole(namespace, name)
	if err != nil {
		return err
	}

	return nil
}

// GrantAppAdminAccessToMachineUserToken Granter function to grant access to service account token
func GrantAppAdminAccessToMachineUserToken(kubeutil *kube.Kube, app *v1.RadixRegistration, namespace string, serviceAccount *corev1.ServiceAccount) error {
	if len(serviceAccount.Secrets) == 0 {
		return fmt.Errorf("Service account %s currently has no secrets associated with it", serviceAccount.Name)
	}

	role := roleAppAdminMachineUserToken(app.Name, serviceAccount)
	err := kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	adGroups, _ := GetAdGroups(app)
	rolebinding := rolebindingAppAdminToMachineUserToken(app.Name, adGroups, role)
	return kubeutil.ApplyRoleBinding(namespace, rolebinding)
}
