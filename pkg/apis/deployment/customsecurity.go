package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

func (deploy *Deployment) customSecuritySettings(appName, namespace string, deployment *v1beta1.Deployment) {
	// need to be able to get serviceaccount token inside container
	automountServiceAccountToken := true
	ownerReference := application.GetOwnerReferenceOfRegistration(deploy.registration)
	if isRadixWebHook(deploy.registration.Namespace, appName) {
		serviceAccountName := "radix-github-webhook"
		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			log.Warnf("Service account for running radix github webhook not made. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", serviceAccount, ownerReference)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
	if isRadixAPI(deploy.registration.Namespace, appName) {
		serviceAccountName := "radix-api"
		serviceAccount, err := deploy.kubeutil.ApplyServiceAccount(serviceAccountName, namespace)
		if err != nil {
			log.Warnf("Error creating Service account for radix api. %v", err)
		} else {
			_ = deploy.kubeutil.ApplyClusterRoleToServiceAccount("radix-operator", serviceAccount, ownerReference)
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}
}

func isRadixAPI(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-api" && radixRegistrationNamespace == corev1.NamespaceDefault
}

func isRadixWebHook(radixRegistrationNamespace, appName string) bool {
	return appName == "radix-github-webhook" && radixRegistrationNamespace == corev1.NamespaceDefault
}
