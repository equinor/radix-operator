package models

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// AlertConfigList list of AlertConfig
type AlertConfigList []AlertConfig

// AsRadixAlertAlerts converts list of AlertConfigs to list of alerts to be used in RadixAlert spec
func (l AlertConfigList) AsRadixAlertAlerts() []radixv1.Alert {
	var alerts []radixv1.Alert

	for _, alertConfig := range l {
		alerts = append(alerts, radixv1.Alert{Alert: alertConfig.Alert, Receiver: alertConfig.Receiver})
	}

	return alerts
}

// AlertConfig defines a mapping between a pre-defined alert name and a receiver
type AlertConfig struct {
	// Receiver is the name of the receiver that will handle this alert
	// required: true
	Receiver string `json:"receiver"`

	// Alert defines the name of a predefined alert
	// required: true
	Alert string `json:"alert"`
}
