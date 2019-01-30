package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createDeployment(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	appName := deploy.radixDeployment.Spec.AppName
	deployment := deploy.getDeploymentConfig(deployComponent)

	deploy.customSecuritySettings(appName, namespace, deployment)
	return deploy.kubeutil.ApplyDeployment(namespace, deployment)
}

func (deploy *Deployment) getDeploymentConfig(deployComponent v1.RadixDeployComponent) *v1beta1.Deployment {
	appName := deploy.radixDeployment.Spec.AppName
	environment := deploy.radixDeployment.Spec.Environment
	componentName := deployComponent.Name
	componentPorts := deployComponent.Ports
	replicas := deployComponent.Replicas
	automountServiceAccountToken := false

	const branchKey, commitIDKey = "radix-branch", "radix-commit"
	rdLabels := deploy.radixDeployment.Labels
	var branch, commitID string
	if branchVal, exists := rdLabels[branchKey]; exists {
		branch = branchVal
	}
	if commitIDVal, exists := rdLabels[commitIDKey]; exists {
		commitID = commitIDVal
	}

	ownerReference := kube.GetOwnerReferenceOfDeploymentWithName(componentName, deploy.radixDeployment)
	securityContext := getSecurityContextForContainer()

	deployment := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"radixApp":               appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixBranchLabel:    branch,
				kube.RadixCommitLabel:    commitID,
			},
			OwnerReferences: ownerReference,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: int32Ptr(DefaultReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixComponentLabel: componentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"radixApp":               appName, // For backwards compatibility. Remove when cluster is migrated
						kube.RadixAppLabel:       appName,
						kube.RadixComponentLabel: componentName,
						kube.RadixBranchLabel:    branch,
						kube.RadixCommitLabel:    commitID,
					},
					Annotations: map[string]string{
						"apparmor.security.beta.kubernetes.io/pod": "runtime/default",
						"seccomp.security.alpha.kubernetes.io/pod": "docker/default",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Containers: []corev1.Container{
						{
							Name:            componentName,
							Image:           deployComponent.Image,
							SecurityContext: securityContext,
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "radix-docker",
						},
					},
				},
			},
		},
	}

	var ports []corev1.ContainerPort
	for _, v := range componentPorts {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: int32(v.Port),
		}
		ports = append(ports, containerPort)
	}
	deployment.Spec.Template.Spec.Containers[0].Ports = ports

	if replicas > 0 {
		deployment.Spec.Replicas = int32Ptr(int32(replicas))
	}

	environmentVariables := deploy.getEnvironmentVariables(deployComponent.EnvironmentVariables, deployComponent.Secrets, deployComponent.Public, deployComponent.Ports, deploy.radixDeployment.Name, deploy.radixDeployment.Namespace, environment, appName, componentName)

	if environmentVariables != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = environmentVariables
	}

	resourceRequirements := getResourceRequirements(deployComponent)

	if resourceRequirements != nil {
		deployment.Spec.Template.Spec.Containers[0].Resources = *resourceRequirements
	}

	return deployment
}

func getSecurityContextForContainer() *corev1.SecurityContext {
	allowPrivilegeEscalation := false
	// runAsNonRoot := true
	// runAsUser := int64(1000)

	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		// RunAsNonRoot:             &runAsNonRoot,
		// RunAsUser:                &runAsUser,
	}
}

func getResourceRequirements(deployComponent v1.RadixDeployComponent) *corev1.ResourceRequirements {
	// if you only set limit, it will use the same values for request
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}
	for name, limit := range deployComponent.Resources.Limits {
		limits[corev1.ResourceName(name)], _ = resource.ParseQuantity(limit)
	}
	for name, req := range deployComponent.Resources.Requests {
		requests[corev1.ResourceName(name)], _ = resource.ParseQuantity(req)
	}

	if len(limits) <= 0 && len(requests) <= 0 {
		return nil
	}

	req := corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}

	return &req
}

func int32Ptr(i int32) *int32 {
	return &i
}
