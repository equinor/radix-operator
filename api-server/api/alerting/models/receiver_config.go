package models

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// ReceiverConfigMap defines a map of ReceiverConfig where key is the name of the receiver
type ReceiverConfigMap map[string]ReceiverConfig

// AsRadixAlertReceiverMap converts map of ReceiverConfigs to map of Receivers to be used in RadixAlert spec
func (m ReceiverConfigMap) AsRadixAlertReceiverMap() radixv1.ReceiverMap {
	receiverMap := make(radixv1.ReceiverMap)

	for receiverName, receiver := range m {
		receiverMap[receiverName] = radixv1.Receiver{
			SlackConfig: radixv1.SlackConfig{
				Enabled: receiver.SlackConfig.Enabled,
			},
		}
	}

	return receiverMap
}

// ReceiverConfig receiver configuration
type ReceiverConfig struct {
	// SlackConfig defines Slack configuration options for this receiver
	// required: true
	SlackConfig *SlackConfig `json:"slackConfig,omitempty"`
}

// SlackConfig configuration options for Slack
type SlackConfig struct {
	// Enabled flag indicates if alert notifications should be sent to Slack
	//
	// required: true
	Enabled bool `json:"enabled"`
}

// ReceiverConfigSecretStatusMap defines a map of ReceiverConfigSecretStatus where key is the name of the receiver
type ReceiverConfigSecretStatusMap map[string]ReceiverConfigSecretStatus

type ReceiverConfigSecretStatus struct {
	SlackConfig *SlackConfigSecretStatus `json:"slackConfig,omitempty"`
}

// SlackConfigSecretStatus
type SlackConfigSecretStatus struct {
	// WebhookURLConfigured flag indicates if a Slack webhook URL is set
	WebhookURLConfigured bool `json:"webhookUrlConfigured"`
}
