package batch

import (
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *syncer) reconcileService(batchJob *radixv1.RadixBatchJob, rd *radixv1.RadixDeployment, jobComponent *radixv1.RadixDeployJobComponent, existingServices []*corev1.Service) error {
	if len(jobComponent.GetPorts()) == 0 {
		return nil
	}

	if isBatchJobStopRequested(batchJob) || isBatchJobDone(s.radixBatch, batchJob.Name) {
		return nil
	}

	if slice.Any(existingServices, func(service *corev1.Service) bool { return isResourceLabeledWithBatchJobName(batchJob.Name, service) }) {
		return nil
	}

	service := s.buildService(batchJob.Name, rd.Spec.AppName, jobComponent.GetPorts())
	return s.kubeUtil.ApplyService(s.radixBatch.GetNamespace(), service)
}

func (s *syncer) buildService(batchJobName, appName string, componentPorts []radixv1.ComponentPort) *corev1.Service {
	serviceName := getKubeServiceName(s.radixBatch.GetName(), batchJobName)
	labels := s.batchJobIdentifierLabel(batchJobName, appName)
	selector := s.batchJobIdentifierLabel(batchJobName, appName)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Labels:          labels,
			OwnerReferences: ownerReference(s.radixBatch),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    utils.GetServicePorts(componentPorts),
			Selector: selector,
		},
	}

	return service
}
