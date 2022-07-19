package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigureDeploymentRbacFunc defines a function that configures RBAC
type ConfigureDeploymentRbacFunc func() error

//GetDeploymentRbacConfigurators returns an array of RBAC configuration functions
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
		newServiceAccount := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.RadixAPIRoleName,
				Namespace: deploy.radixDeployment.Namespace,
			},
		}

		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(newServiceAccount)
		if err != nil {
			log.Warnf("Error creating Service account for radix api. %v", err)
			return err
		}
		return deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-api", serviceAccount, ownerReference)
	}
}

func configureRbacForRadixGithubWebhook(deploy *Deployment) ConfigureDeploymentRbacFunc {
	ownerReference := application.GetOwnerReferenceOfRegistration(deploy.registration)

	return func() error {
		newServiceAccount := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.RadixGithubWebhookRoleName,
				Namespace: deploy.radixDeployment.Namespace,
			},
		}

		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(newServiceAccount)
		if err != nil {
			log.Warnf("Service account for running radix github webhook not made. %v", err)
			return err
		}
		return deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-webhook", serviceAccount, ownerReference)
	}
}

func configureRbacForRadixJobComponents(deploy *Deployment) ConfigureDeploymentRbacFunc {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName

	return func() error {
		newServiceAccount := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.RadixJobSchedulerServerServiceName,
				Namespace: namespace,
			},
		}

		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(newServiceAccount)
		if err != nil {
			log.Warnf("Error creating Service account for radix job scheduler. %v", err)
			return err
		}
		subjects := []auth.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}}

		roleBinding := kube.GetRolebindingToClusterRoleForSubjects(appName, defaults.RadixJobSchedulerServerRoleName, subjects)
		return deploy.kubeutil.ApplyRoleBinding(namespace, roleBinding)
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
