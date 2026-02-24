package alert

import (
	"context"
	"encoding/json"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// For each alertmanmagerconfig we need a receiver that does nothing at the root route.
	// All real alerts and receivers are configured as childs routes to the root.
	noopRecevierName = "noop"
	// Repeat interval for non-resolvable alerts should be equal to
	// alertmanagers alert retention period.
	nonResolvableRepeatInterval = "120h"
	// How long to wait before sending a notification again if it has already
	// been sent successfully for an alert.
	resolvableRepeatInterval = "24h"
	// How long to wait before sending a notification about new alerts that
	// are added to a group of alerts for which an initial notification has
	// # already been sent.
	defaultGroupInterval = "1m"
	// How long to initially wait to send a notification for a group
	// of alerts. Allows to wait for an inhibiting alert to arrive or collect
	// more initial alerts for the same group.
	defaultGroupWait = "30s"
)

type alertConfigList []AlertConfig

func (list alertConfigList) Any(anyFunc func(c AlertConfig) bool) bool {
	for _, alertConfig := range list {
		if anyFunc(alertConfig) {
			return true
		}
	}

	return false
}

func (syncer *alertSyncer) reconcileAlertManagerConfig(ctx context.Context) error {
	ns := syncer.radixAlert.Namespace

	config := &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(syncer.radixAlert.Name), Namespace: ns}}
	op, err := controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, config, func() error {
		receivers := syncer.getAlertmanagerConfigReceivers()
		routes := syncer.getAlertmanagerConfigRoutes(ctx)

		routeJSON := []apiextensionsv1.JSON{}
		for _, route := range routes {
			routeBytes, err := json.Marshal(route)
			if err != nil {
				return err
			}
			routeJSON = append(routeJSON, apiextensionsv1.JSON{Raw: routeBytes})
		}

		config.Spec = v1alpha1.AlertmanagerConfigSpec{
			Receivers: receivers,
			Route: &v1alpha1.Route{
				Receiver: noopRecevierName,
				Routes:   routeJSON,
			},
		}
		return controllerutil.SetControllerReference(syncer.radixAlert, config, syncer.dynamicClient.Scheme())
	})

	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("operation", string(op)).Msg("recconcile alert manager config")
	}

	return err
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

	if alertConfigs.Any(func(c AlertConfig) bool { return c.Resolvable }) {
		alertmanagerConfigReceivers = append(alertmanagerConfigReceivers, syncer.buildAlertmanagerConfigReceiver(receiver, name, true))
	}

	if alertConfigs.Any(func(c AlertConfig) bool { return !c.Resolvable }) {
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
				Text:      syncer.slackMessageTemplate.text,
				Title:     syncer.slackMessageTemplate.title,
				TitleLink: syncer.slackMessageTemplate.titleLink,
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

func (syncer *alertSyncer) getAlertmanagerConfigRoutes(ctx context.Context) []v1alpha1.Route {
	var routes []v1alpha1.Route

	for _, alert := range syncer.radixAlert.Spec.Alerts {
		alertConfig, found := syncer.alertConfigs[alert.Alert]
		if !found {
			log.Ctx(ctx).Debug().Msgf("skipping unknown alert %s in RadixAlert %s", alert.Alert, syncer.radixAlert.Name)
			continue
		}
		receiver, found := syncer.radixAlert.Spec.Receivers[alert.Receiver]
		if !found {
			log.Ctx(ctx).Debug().Msgf("skipping alert %s in RadixAlert %s with unknown recevier %s", alert.Alert, syncer.radixAlert.Name, alert.Receiver)
			continue
		}
		if !receiver.IsEnabled() {
			log.Ctx(ctx).Debug().Msgf("skipping alert %s in RadixAlert %s because receiver %s is disabled", alert.Alert, syncer.radixAlert.Name, alert.Receiver)
			continue
		}
		routes = append(routes, v1alpha1.Route{
			Receiver:       getRouteReceiverNameForAlert(alert.Receiver, alertConfig.Resolvable),
			GroupBy:        alertConfig.GroupBy,
			GroupWait:      defaultGroupWait,
			GroupInterval:  defaultGroupInterval,
			RepeatInterval: getRepeatInterval(alertConfig),
			Matchers:       []v1alpha1.Matcher{{Name: "alertname", Value: alert.Alert, MatchType: v1alpha1.MatchEqual}},
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

func getRepeatInterval(alertConfig AlertConfig) string {
	if !alertConfig.Resolvable {
		return nonResolvableRepeatInterval
	}
	return resolvableRepeatInterval
}

func getAlertmanagerConfigName(alertName string) string {
	return alertName
}
