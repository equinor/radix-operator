package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// ConfigureDeploymentRbacFunc defines a function that configures RBAC
type ConfigureDeploymentRbacFunc func() error

// GetDeploymentRbacConfigurators returns an array of RBAC configuration functions
func GetDeploymentRbacConfigurators(ctx context.Context, deploy *Deployment) []ConfigureDeploymentRbacFunc {
	var rbac []ConfigureDeploymentRbacFunc

	if hasJobComponent(deploy.radixDeployment) {
		rbac = append(rbac, configureRbacForRadixJobComponents(ctx, deploy))
	}

	return rbac
}

func configureRbacForRadixJobComponents(ctx context.Context, deploy *Deployment) ConfigureDeploymentRbacFunc {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName //nolint:staticcheck

	return func() error {
		serviceAccount, err := deploy.kubeutil.CreateServiceAccount(ctx, namespace, defaults.RadixJobSchedulerServiceName)
		if err != nil {
			return fmt.Errorf("failed to create service account for radix job scheduler: %w", err)
		}
		subjects := []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}}

		envRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(appName, defaults.RadixJobSchedulerRoleName, subjects)
		return deploy.kubeutil.ApplyRoleBinding(ctx, namespace, envRoleBinding)
	}
}

func hasJobComponent(rd *v1.RadixDeployment) bool {
	return len(rd.Spec.Jobs) > 0
}
