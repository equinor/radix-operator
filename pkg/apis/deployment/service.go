package deployment

import (
	"context"
	"fmt"

	internal "github.com/equinor/radix-operator/pkg/apis/internal/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateService(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	ports := deployComponent.GetPorts()
	if len(ports) == 0 {
		return nil
	}
	service := getServiceConfig(deployComponent, deploy.radixDeployment, ports)
	return deploy.kubeutil.ApplyService(ctx, namespace, service)
}

func (deploy *Deployment) garbageCollectServicesNoLongerInSpec(ctx context.Context) error {
	services, err := deploy.kubeutil.ListServices(ctx, deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, service := range services {
		componentName, ok := RadixComponentNameFromComponentLabel(service)
		if !ok {
			continue
		}
		if deploy.isEligibleForGarbageCollectServiceForComponent(service, componentName) {
			if err := deploy.kubeclient.CoreV1().Services(deploy.radixDeployment.GetNamespace()).Delete(ctx, service.Name, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) isEligibleForGarbageCollectServiceForComponent(service *corev1.Service, componentName RadixComponentName) bool {
	if jobType, ok := NewRadixJobTypeFromObjectLabels(service); ok && jobType.IsJobScheduler() {
		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			return true // Garbage collect if service is labelled radix-job-type=job-scheduler and not defined in RD jobs
		}
	} else if !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
		return true // Garbage collect service if not defined in RD components or jobs
	}

	return !componentName.CommonDeployComponentHasPorts(deploy.radixDeployment)
}

func getServiceConfig(component v1.RadixCommonDeployComponent, radixDeployment *v1.RadixDeployment, componentPorts []v1.ComponentPort) *corev1.Service {
	ownerReference := []metav1.OwnerReference{
		getOwnerReferenceOfDeployment(radixDeployment),
	}

	selector := map[string]string{kube.RadixComponentLabel: component.GetName()}
	if internal.IsDeployComponentJobSchedulerDeployment(component) {
		selector[kube.RadixPodIsJobSchedulerLabel] = "true"
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
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports:    utils.GetServicePorts(componentPorts),
		},
	}

	return service
}

// GetAuxOAuthRedisServiceName returns service name for auxiliary OAuth redis component
func GetAuxOAuthRedisServiceName(componentName string) string {
	return fmt.Sprintf("%s-%s", componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
}
