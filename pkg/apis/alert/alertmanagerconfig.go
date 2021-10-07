package alert

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	noopRecevierName            = "noop"
	nonResolvableRepeatInterval = "120h"
	defaultRepeatInterval       = "24h"
	defaultGroupInterval        = "5m"
	defaultGroupWait            = "30s"
	radixApplicationNameLabel   = "label_radix_app"
	radixEnvironmentNameLabel   = "label_radix_component"
	radixComponentNameLabel     = "label_radix_env"
	radixJobNameLabel           = "label_radix_job_name"
)

type alertConfigList []alertConfig

func (list alertConfigList) Any(anyFunc func(c alertConfig) bool) bool {
	for _, alertConfig := range list {
		if anyFunc(alertConfig) {
			return true
		}
	}

	return false
}

func (syncer *alertSyncer) createOrUpdateAlertManagerConfig() error {
	ns := syncer.radixAlert.Namespace
	amc, err := syncer.getAlertManagerConfig()
	if err != nil {
		return err
	}
	syncer.applyAlertManagerConfig(ns, amc)
	return err
}

func (syncer *alertSyncer) applyAlertManagerConfig(namespace string, alertManagerConfig *v1alpha1.AlertmanagerConfig) error {
	oldConfig, err := syncer.prometheusperatorclient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.TODO(), alertManagerConfig.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			created, err := syncer.prometheusperatorclient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Create(context.TODO(), alertManagerConfig, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create AlertManagerConfig object: %v", err)
			}

			syncer.logger.Debugf("Created AlertManagerConfig: %s in namespace %s", created.Name, namespace)
			return nil
		}
		return err
	}

	oldConfigJSON, err := json.Marshal(oldConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal old AlertManagerConfig object: %v", err)
	}

	// Avoid uneccessary patching
	newConfig := oldConfig.DeepCopy()
	newConfig.Annotations = alertManagerConfig.Annotations
	newConfig.Labels = alertManagerConfig.Labels
	newConfig.OwnerReferences = alertManagerConfig.OwnerReferences
	newConfig.Spec = alertManagerConfig.Spec

	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal new AlertManagerConfig object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldConfigJSON, newConfigJSON, v1alpha1.AlertmanagerConfig{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch AlertManagerConfig objects: %v", err)
	}

	if !kube.IsEmptyPatch(patchBytes) {
		// Will perform update as patching does not work
		updatedConfig, err := syncer.prometheusperatorclient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Update(context.TODO(), newConfig, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update AlertManagerConfig object: %v", err)
		}

		syncer.logger.Debugf("Updated AlertManagerConfig: %s ", updatedConfig.Name)
		return nil

	}

	return nil
}

func (syncer *alertSyncer) getAlertManagerConfig() (*v1alpha1.AlertmanagerConfig, error) {
	receivers := syncer.getAlertmanagerConfigReceivers()
	routeJSON := []apiextensionsv1.JSON{}
	routes := syncer.getAlertmanagerConfigRoutes()
	for _, route := range routes {
		routeBytes, err := json.Marshal(route)
		if err != nil {
			return nil, err
		}
		routeJSON = append(routeJSON, apiextensionsv1.JSON{Raw: routeBytes})
	}

	amc := &v1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-amc", syncer.radixAlert.Name),
			OwnerReferences: syncer.getOwnerReference(),
		},
		Spec: v1alpha1.AlertmanagerConfigSpec{
			Receivers: receivers,
			Route: &v1alpha1.Route{
				Receiver: noopRecevierName,
				Routes:   routeJSON,
			},
		},
	}

	return amc, nil
}

func (syncer *alertSyncer) getAlertmanagerConfigReceivers() []v1alpha1.Receiver {
	receivers := []v1alpha1.Receiver{{Name: noopRecevierName}}

	for name, receiver := range syncer.radixAlert.Spec.Receivers {
		receivers = append(receivers, syncer.getAlertmanagerConfigReceiverForRadixAlertReceiver(name, &receiver)...)
	}

	return receivers
}

func (syncer *alertSyncer) getAlertmanagerConfigReceiverForRadixAlertReceiver(name string, receiver *radixv1.Receiver) []v1alpha1.Receiver {
	var alertmanagerConfigReceivers []v1alpha1.Receiver

	if !receiver.IsEnabled() {
		return alertmanagerConfigReceivers
	}

	alertConfigs := syncer.getMappedAlertConfigsForReceiverName(name)

	if alertConfigs.Any(func(c alertConfig) bool { return c.resolvable }) {
		alertmanagerConfigReceivers = append(alertmanagerConfigReceivers, syncer.buildAlertmanagerConfigReceiver(receiver, name, true))
	}

	if alertConfigs.Any(func(c alertConfig) bool { return !c.resolvable }) {
		alertmanagerConfigReceivers = append(alertmanagerConfigReceivers, syncer.buildAlertmanagerConfigReceiver(receiver, name, false))
	}

	return alertmanagerConfigReceivers
}

func (syncer *alertSyncer) buildAlertmanagerConfigReceiver(receiver *radixv1.Receiver, receiverName string, resolvable bool) v1alpha1.Receiver {
	slackConfigsBuilder := func() []v1alpha1.SlackConfig {
		if !receiver.SlackConfig.Enabled {
			return nil
		}

		return []v1alpha1.SlackConfig{
			{
				SendResolved: &resolvable,
				APIURL: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: GetAlertSecretName(syncer.radixAlert.Name)},
					Key:                  GetSlackConfigSecretKeyName(receiverName),
				},
				Text:      &syncer.slackMessageTemplate.text,
				Title:     &syncer.slackMessageTemplate.title,
				TitleLink: &syncer.slackMessageTemplate.titleLink,
			},
		}
	}

	return v1alpha1.Receiver{
		Name:         getRouteReceiverNameForAlert(receiverName, resolvable),
		SlackConfigs: slackConfigsBuilder(),
	}
}

func (syncer *alertSyncer) getMappedAlertConfigsForReceiverName(receiverName string) alertConfigList {
	var mappedAlertConfigs alertConfigList

	for _, alert := range syncer.radixAlert.Spec.Alerts {
		if alertConfig, found := syncer.alertConfigs[alert.Alert]; found && alert.Receiver == receiverName {
			mappedAlertConfigs = append(mappedAlertConfigs, alertConfig)
		}
	}

	return mappedAlertConfigs
}

func (syncer *alertSyncer) getAlertmanagerConfigRoutes() []v1alpha1.Route {
	var routes []v1alpha1.Route

	for _, alert := range syncer.radixAlert.Spec.Alerts {
		alertConfig, found := syncer.alertConfigs[alert.Alert]
		if !found {
			syncer.logger.Debugf("skipping unknown alert %s in RadixAlert %s", alert.Alert, syncer.radixAlert.Name)
			continue
		}
		receiver, found := syncer.radixAlert.Spec.GetReceiverByName(alert.Receiver)
		if !found {
			syncer.logger.Debugf("skipping alert %s in RadixAlert %s with unknown recevier %s", alert.Alert, syncer.radixAlert.Name, alert.Receiver)
			continue
		}
		if !receiver.IsEnabled() {
			syncer.logger.Debugf("skipping alert %s in RadixAlert %s because receiver %s is disabled", alert.Alert, syncer.radixAlert.Name, alert.Receiver)
			continue
		}
		routes = append(routes, v1alpha1.Route{
			Receiver:       getRouteReceiverNameForAlert(alert.Receiver, alertConfig.resolvable),
			GroupBy:        alertConfig.groupBy,
			GroupWait:      defaultGroupWait,
			GroupInterval:  defaultGroupInterval,
			RepeatInterval: getRepeatInterval(alertConfig),
			Matchers:       []v1alpha1.Matcher{{Name: "alertname", Value: alert.Alert}},
		})
	}

	return routes
}

func getRouteReceiverNameForAlert(receiverName string, resolvable bool) string {
	if resolvable {
		return fmt.Sprintf("%s-%s", receiverName, "resolvable")
	}

	return receiverName
}

func getRepeatInterval(alertConfig alertConfig) string {
	if !alertConfig.resolvable {
		return nonResolvableRepeatInterval
	}
	return defaultRepeatInterval
}
