package scheduledjob

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s *syncer) reconcileService() error {
	rd, jobComponent, err := s.getRadixDeploymentAndJobComponent()
	if err != nil {
		return err
	}
	if len(jobComponent.GetPorts()) == 0 {
		return nil
	}
	selector := s.scheduledJobIdentifierLabel()
	existingServices, err := s.kubeclient.CoreV1().Services(s.radixScheduledJob.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}
	if len(existingServices.Items) > 0 {
		return nil
	}
	serviceName := s.radixScheduledJob.GetName()
	service := s.buildService(serviceName, jobComponent.Name, rd.Spec.AppName, jobComponent.GetPorts())
	return s.kubeutil.ApplyService(s.radixScheduledJob.GetNamespace(), service)

}

func (s *syncer) buildService(serviceName, componentName, appName string, componentPorts []v1.ComponentPort) *corev1.Service {
	labels := radixlabels.Merge(
		s.scheduledJobIdentifierLabel(),
		radixlabels.ForApplicationName(appName),
		radixlabels.ForComponentName(componentName),
		radixlabels.ForJobType(kube.RadixJobTypeJobSchedule),
	)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Labels:          labels,
			OwnerReferences: ownerReference(s.radixScheduledJob),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    buildServicePorts(componentPorts),
			Selector: s.scheduledJobIdentifierLabel(),
		},
	}

	return service
}

func buildServicePorts(componentPorts []v1.ComponentPort) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Name:       v.Name,
			Port:       v.Port,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(int(v.Port)),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
