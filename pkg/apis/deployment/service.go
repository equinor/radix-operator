package deployment

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) createOrUpdateService(deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	service := getServiceConfig(deployComponent, deploy.radixDeployment, deployComponent.GetPorts())
	return deploy.kubeutil.ApplyService(namespace, service)
}

func (deploy *Deployment) garbageCollectServicesNoLongerInSpec() error {
	services, err := deploy.kubeutil.ListServices(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, service := range services {
		componentName, ok := RadixComponentNameFromComponentLabel(service)
		if !ok {
			continue
		}
		if deploy.isEligibleForGarbageCollectServiceForComponent(service, componentName) {
			err = deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) isEligibleForGarbageCollectServiceForComponent(service *corev1.Service, componentName RadixComponentName) bool {
	// Garbage collect if service is labelled radix-job-type=job-scheduler and not defined in RD jobs
	if jobType, ok := NewRadixJobTypeFromObjectLabels(service); ok && jobType.IsJobScheduler() {
		return !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment)
	}
	// Garbage collect service if not defined in RD components or jobs
	return !componentName.ExistInDeploymentSpec(deploy.radixDeployment)
}

func getServiceConfig(component v1.RadixCommonDeployComponent, radixDeployment *v1.RadixDeployment, componentPorts []v1.ComponentPort) *corev1.Service {
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(radixDeployment),
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: component.GetName(),
			Labels: map[string]string{
				kube.RadixAppLabel:       radixDeployment.Spec.AppName,
				kube.RadixComponentLabel: component.GetName(),
			},
			OwnerReferences: ownerReference,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	selector := map[string]string{kube.RadixComponentLabel: component.GetName()}
	if isDeployComponentJobSchedulerDeployment(component) {
		selector[kube.RadixPodIsJobSchedulerLabel] = "true"
	}
	service.Spec.Selector = selector

	ports := buildServicePorts(componentPorts)
	service.Spec.Ports = ports

	return service
}

func buildServicePorts(componentPorts []v1.ComponentPort) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Name:       v.Name,
			Port:       int32(v.Port),
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(int(v.Port)),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
