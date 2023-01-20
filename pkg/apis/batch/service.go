package batch

import (
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s *syncer) reconcileServices(rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent) error {
	if len(jobComponent.GetPorts()) == 0 {
		return nil
	}

	existingServices, err := s.kubeutil.ListServicesWithSelector(s.batch.GetNamespace(), s.batchIdentifierLabel().String())
	if err != nil {
		return err
	}

	for _, batchjob := range s.batch.Spec.Jobs {
		if isBatchJobStopRequested(batchjob) {
			continue
		}

		if !slice.Any(existingServices, func(service *corev1.Service) bool { return isResourceLabeledWithBatchJobName(batchjob.Name, service) }) {
			service := s.buildService(batchjob.Name, jobComponent.GetPorts())
			if err := s.kubeutil.ApplyService(s.batch.GetNamespace(), service); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *syncer) buildService(batchJobName string, componentPorts []radixv1.ComponentPort) *corev1.Service {
	serviceName := getKubeServiceName(s.batch.GetName(), batchJobName)
	labels := radixlabels.Merge(
		s.batchIdentifierLabel(),
		radixlabels.ForBatchJobName(batchJobName),
	)
	selector := radixlabels.Merge(
		s.batchIdentifierLabel(),
		radixlabels.ForBatchJobName(batchJobName),
	)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Labels:          labels,
			OwnerReferences: ownerReference(s.batch),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    buildServicePorts(componentPorts),
			Selector: selector,
		},
	}

	return service
}

func buildServicePorts(componentPorts []radixv1.ComponentPort) []corev1.ServicePort {
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
