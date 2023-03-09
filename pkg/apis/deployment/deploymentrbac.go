package deployment

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
)

// ConfigureDeploymentRbacFunc defines a function that configures RBAC
type ConfigureDeploymentRbacFunc func() error

// GetDeploymentRbacConfigurators returns an array of RBAC configuration functions
func GetDeploymentRbacConfigurators(deploy *Deployment) []ConfigureDeploymentRbacFunc {
	var rbac []ConfigureDeploymentRbacFunc

	if isRadixAPI(deploy.radixDeployment) {
		rbac = append(rbac, configureRbacForRadixAPI(deploy))
	}

	if isRadixWebHook(deploy.radixDeployment) {
		rbac = append(rbac, configureRbacForRadixGithubWebhook(deploy))
	}

	if hasJobcomponent(deploy.radixDeployment) {
		rbac = append(rbac, configureRbacForRadixJobComponents(deploy))
	}

	return rbac
}

func configureRbacForRadixAPI(deploy *Deployment) ConfigureDeploymentRbacFunc {
	ownerReference := application.GetOwnerReferenceOfRegistration(deploy.registration)

	return func() error {
		serviceAccount, err := deploy.kubeutil.CreateServiceAccount(deploy.radixDeployment.Namespace, defaults.RadixAPIRoleName)
		if err != nil {
			return fmt.Errorf("error creating Service account for radix api. %v", err)
		}
		return deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-api", serviceAccount, ownerReference)
	}
}

func configureRbacForRadixGithubWebhook(deploy *Deployment) ConfigureDeploymentRbacFunc {
	ownerReference := application.GetOwnerReferenceOfRegistration(deploy.registration)

	return func() error {
		serviceAccount, err := deploy.kubeutil.CreateServiceAccount(deploy.radixDeployment.Namespace, defaults.RadixGithubWebhookRoleName)
		if err != nil {
			return fmt.Errorf("service account for running radix github webhook not made. %v", err)
		}
		return deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-webhook", serviceAccount, ownerReference)
	}
}

func configureRbacForRadixJobComponents(deploy *Deployment) ConfigureDeploymentRbacFunc {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName

	return func() error {
		serviceAccount, err := deploy.kubeutil.CreateServiceAccount(namespace, defaults.RadixJobSchedulerServiceName)
		if err != nil {
			return fmt.Errorf("error creating Service account for radix job scheduler. %v", err)
		}
		subjects := []auth.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}}

		envRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(appName, defaults.RadixJobSchedulerEnvRoleName, subjects)
		err = deploy.kubeutil.ApplyRoleBinding(namespace, envRoleBinding)
		if err != nil {
			return err
		}
		appRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(appName, defaults.RadixJobSchedulerAppRoleName, subjects)
		return deploy.kubeutil.ApplyRoleBinding(utils.GetAppNamespace(appName), appRoleBinding)
	}
}

func isRadixAPI(rd *v1.RadixDeployment) bool {
	return rd.Spec.AppName == "radix-api"
}

func isRadixWebHook(rd *v1.RadixDeployment) bool {
	return rd.Spec.AppName == "radix-github-webhook"
}

func hasJobcomponent(rd *v1.RadixDeployment) bool {
	return len(rd.Spec.Jobs) > 0
}
