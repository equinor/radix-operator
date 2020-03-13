package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) customSecuritySettings(appName, namespace string, deployment *v1beta1.Deployment) {
	// need to be able to get serviceaccount token inside container
	automountServiceAccountToken := true
	ownerReference := application.GetOwnerReferenceOfRegistration(deploy.registration)
	if isRadixWebHook(appName) {
		newServiceAccount := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.RadixGithubWebhookRoleName,
				Namespace: namespace,
			},
		}

		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(newServiceAccount)
		if err != nil {
			log.Warnf("Service account for running radix github webhook not made. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-webhook", serviceAccount, ownerReference)
			deployment.Spec.Template.Spec.ServiceAccountName = defaults.RadixGithubWebhookRoleName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
	if isRadixAPI(appName) {
		newServiceAccount := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.RadixAPIRoleName,
				Namespace: namespace,
			},
		}

		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(newServiceAccount)
		if err != nil {
			log.Warnf("Error creating Service account for radix api. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-api", serviceAccount, ownerReference)
			deployment.Spec.Template.Spec.ServiceAccountName = defaults.RadixAPIRoleName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
}

func isRadixAPI(appName string) bool {
	return appName == "radix-api"
}

func isRadixWebHook(appName string) bool {
	return appName == "radix-github-webhook"
}
