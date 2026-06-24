package models

// AlertingConfig current alert settings
// swagger:model AlertingConfig
type AlertingConfig struct {
	// Enabled flag tells if alerting is enabled or disabled
	Enabled bool `json:"enabled"`

	// Ready flag tells tells if alerting is ready to be configured
	// Value is always false when Enabled is false
	// Vlaue is True if Enabled is true and Radix operator has processed the alert configuration
	Ready bool `json:"ready"`

	// Receivers map of available receivers to be mapped to alerts
	Receivers ReceiverConfigMap `json:"receivers,omitempty"`

	// ReceiverSecretStatus has status of required secrets for each receiver
	ReceiverSecretStatus ReceiverConfigSecretStatusMap `json:"receiverSecretStatus,omitempty"`

	// Alerts is the list of configured alerts
	Alerts AlertConfigList `json:"alerts,omitempty"`

	// AlertNames is the list of alert names that can be handled by Radix
	AlertNames []string `json:"alertNames,omitempty"`
}
