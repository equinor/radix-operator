package deployment

import (
	"fmt"
	"os"
	"strings"

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

func (deploy *Deployment) garbageCollectServiceMonitorsNoLongerInSpec() error {
	if deploy.prometheusperatorclient == nil {
		// For testing we need to set the client to nil, to avvoid this code from being executed, due to not having any fake
		return nil
	}

	serviceMonitors, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	sms := serviceMonitors.(*monitoringv1.ServiceMonitorList)

	for _, exisitingComponent := range sms.Items {
		garbageCollect := true
		exisitingComponentName := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
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
		log.Debugf("ServiceMonitor object %s already exists in namespace %s, updating the object now", serviceMonitor.Name, namespace)
		updatedServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Update(serviceMonitor)
		if err != nil {
			return fmt.Errorf("Failed to update ServiceMonitor object: %v", err)
		}
		log.Debugf("Updated ServiceMonitor: %s in namespace %s", updatedServiceMonitor.Name, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Failed to create ServiceMonitor object: %v", err)
	}
	log.Debugf("Created ServiceMonitor: %s in namespace %s", createdServiceMonitor.Name, namespace)
	return nil
}
