package deployment

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) createService(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	service := getServiceConfig(deployComponent.Name, deploy.radixDeployment, deployComponent.Ports)
	return deploy.kubeutil.ApplyService(namespace, service)
}

func (deploy *Deployment) garbageCollectServicesNoLongerInSpec() error {
	services, err := deploy.kubeutil.ListServices(deploy.radixDeployment.GetNamespace())

	for _, exisitingComponent := range services {
		garbageCollect := true
		exisitingComponentName := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getServiceConfig(componentName string, radixDeployment *v1.RadixDeployment, componentPorts []v1.ComponentPort) *corev1.Service {
	ownerReference := getOwnerReferenceOfDeployment(radixDeployment)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixAppLabel:       radixDeployment.Spec.AppName,
				kube.RadixComponentLabel: componentName,
			},
			OwnerReferences: ownerReference,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
	}

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
