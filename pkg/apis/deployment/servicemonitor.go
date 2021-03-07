package deployment

import (
	"fmt"
	"os"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) createOrUpdateServiceMonitor(deployComponent v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	serviceMonitor := getServiceMonitorConfig(deployComponent.GetName(), namespace, deployComponent.GetPorts())
	return deploy.applyServiceMonitor(namespace, serviceMonitor)
}

func (deploy *Deployment) deleteServiceMonitorForComponent(component v1.RadixCommonDeployComponent) error {
	serviceMonitors, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, serviceMonitor := range serviceMonitors.Items {
		componentName, ok := NewRadixComponentNameFromLabels(serviceMonitor)
		if ok && component.GetName() == string(componentName) {
			err = deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).Delete(serviceMonitor.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectServiceMonitorsNoLongerInSpec() error {
	serviceMonitors, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to get ServiceMonitors. Error: %v", err)
		return err
	}

	for _, serviceMonitor := range serviceMonitors.Items {
		// Handle servicemonitors with prometheus=radix_stage1 label only for backward compatibility
		// Code can be removed when all servicemonitors has radix-component label
		garbageCollectBackwardCompatibility := false
		labelValue, ok := serviceMonitor.Labels["prometheus"]
		if ok && labelValue == os.Getenv(prometheusInstanceLabel) && len(serviceMonitor.Labels) == 1 {
			garbageCollectBackwardCompatibility = true
		}

		componentName, ok := NewRadixComponentNameFromLabels(serviceMonitor)
		if !ok {
			continue
		}

		if garbageCollectBackwardCompatibility || !componentName.ExistInDeploymentSpec(deploy.radixDeployment) {
			err = deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(deploy.radixDeployment.GetNamespace()).Delete(serviceMonitor.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getServiceMonitorConfig(componentName, namespace string, componentPorts *[]v1.ComponentPort) *monitoringv1.ServiceMonitor {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: componentName,
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "5s",
					Port:     (*componentPorts)[0].Name,
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

func (deploy *Deployment) getServiceMonitor(namespace, name string) (serviceMonitor *monitoringv1.ServiceMonitor, err error) {
	serviceMonitor, err = deploy.prometheusperatorclient.
		MonitoringV1().
		ServiceMonitors(namespace).
		Get(name, metav1.GetOptions{})
	return
}

func (deploy *Deployment) applyServiceMonitor(namespace string, serviceMonitor *monitoringv1.ServiceMonitor) error {
	serviceMonitorName := serviceMonitor.Name
	oldServiceMonitor, err := deploy.getServiceMonitor(namespace, serviceMonitorName)
	if err != nil && errors.IsNotFound(err) {
		createdServiceMonitor, err := deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Create(serviceMonitor)
		if err != nil {
			return fmt.Errorf("Failed to create ServiceMonitor object: %v", err)
		}

		log.Debugf("Created ServiceMonitor: %s in namespace %s", createdServiceMonitor.Name, namespace)
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get ServiceMonitor object: %v", err)
	}

	newServiceMonitor := oldServiceMonitor.DeepCopy()
	newServiceMonitor.ObjectMeta.Labels = serviceMonitor.Labels
	newServiceMonitor.ObjectMeta.Annotations = serviceMonitor.ObjectMeta.Annotations
	newServiceMonitor.ObjectMeta.OwnerReferences = serviceMonitor.ObjectMeta.OwnerReferences
	newServiceMonitor.Spec = serviceMonitor.Spec

	_, err = deploy.prometheusperatorclient.MonitoringV1().ServiceMonitors(namespace).Update(newServiceMonitor)
	if err != nil {
		return fmt.Errorf("Failed to update ServiceMonitor object: %v", err)
	}

	return nil
}
