package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (deploy *Deployment) createOrUpdateServiceMonitor(ctx context.Context, deployComponent v1.RadixCommonDeployComponent) error {
	monitoringConfig := deployComponent.GetMonitoringConfig()
	if monitoringConfig.PortName == "" {
		ports := deployComponent.GetPorts()
		if len(ports) > 0 {
			monitoringConfig.PortName = ports[0].Name
		}
	}

	namespace := deploy.radixDeployment.Namespace
	componentName := deployComponent.GetName()
	sb := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: componentName, Namespace: namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, sb, func() error {
		if sb.ObjectMeta.Labels == nil {
			sb.ObjectMeta.Labels = map[string]string{}
		}
		sb.ObjectMeta.Labels = map[string]string{
			kube.RadixComponentLabel: componentName,
		}

		sb.Spec = monitoringv1.ServiceMonitorSpec{

			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "5s",
					Path:     monitoringConfig.Path,
					Port:     monitoringConfig.PortName,
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
		}

		sb.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		return controllerutil.SetControllerReference(deploy.radixDeployment, sb, deploy.dynamicClient.Scheme())
	})
	if err != nil {
		return fmt.Errorf("failed to create or update service monitor '%s': %w", componentName, err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("servicemonitor", componentName).Str("op", string(op)).Msg("reconcile service monitor")
	}

	return nil
}

func (deploy *Deployment) deleteServiceMonitorForComponent(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	serviceMonitors := &monitoringv1.ServiceMonitorList{}
	if err := deploy.dynamicClient.List(ctx, serviceMonitors, client.InNamespace(deploy.radixDeployment.GetNamespace())); err != nil {
		return fmt.Errorf("failed to list service monitor  for component: %w", err)
	}

	for _, serviceMonitor := range serviceMonitors.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(serviceMonitor)
		if ok && component.GetName() == string(componentName) {
			if err := deploy.dynamicClient.Delete(ctx, serviceMonitor); err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectServiceMonitorsNoLongerInSpec(ctx context.Context) error {
	serviceMonitors := &monitoringv1.ServiceMonitorList{}
	if err := deploy.dynamicClient.List(ctx, serviceMonitors, client.InNamespace(deploy.radixDeployment.GetNamespace())); err != nil {
		return fmt.Errorf("failed to get ServiceMonitors: %w", err)
	}

	for _, serviceMonitor := range serviceMonitors.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(serviceMonitor)
		if !ok {
			continue
		}
		if deploy.isEligibleForGarbageCollectServiceMonitorsForComponent(serviceMonitor, componentName) {

			if err := deploy.dynamicClient.Delete(ctx, serviceMonitor); err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) isEligibleForGarbageCollectServiceMonitorsForComponent(serviceMonitor *monitoringv1.ServiceMonitor, componentName RadixComponentName) bool {
	return !componentName.ExistInDeploymentSpec(deploy.radixDeployment)
}
