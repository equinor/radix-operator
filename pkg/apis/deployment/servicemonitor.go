package deployment

import (
	"fmt"
	"os"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createServiceMonitor(deployComponent v1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	serviceMonitor := getServiceMonitorConfig(deployComponent.Name, namespace, deployComponent.Ports)
	return deploy.applyServiceMonitor(namespace, serviceMonitor)
}

func getServiceMonitorConfig(componentName, namespace string, componentPorts []v1.ComponentPort) *monitoringv1.ServiceMonitor {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				"prometheus": os.Getenv(prometheusInstanceLabel),
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "5s",
					Port:     componentPorts[0].Name,
				},
			},
			JobLabel: fmt.Sprintf("%s-%s", namespace, componentName),
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					namespace,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixComponentLabel: componentName,
				},
			},
		},
	}
	return serviceMonitor
}

func (deploy *Deployment) applyServiceMonitor(namespace string, serviceMonitor *monitoringv1.ServiceMonitor) error {
	createdServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Create(serviceMonitor)
	if errors.IsAlreadyExists(err) {
		log.Infof("ServiceMonitor object %s already exists in namespace %s, updating the object now", serviceMonitor.Name, namespace)
		updatedServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Update(serviceMonitor)
		if err != nil {
			return fmt.Errorf("Failed to update ServiceMonitor object: %v", err)
		}
		log.Infof("Updated ServiceMonitor: %s in namespace %s", updatedServiceMonitor.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create ServiceMonitor object: %v", err)
	}
	log.Infof("Created ServiceMonitor: %s in namespace %s", createdServiceMonitor.Name, namespace)
	return nil
}
