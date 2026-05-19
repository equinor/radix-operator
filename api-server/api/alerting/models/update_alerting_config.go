package models

// UpdateAlertingConfig contains fields for updating alert settings
// swagger:model UpdateAlertingConfig
type UpdateAlertingConfig struct {
	// Receivers map of receivers
	//
	// required: true
	Receivers ReceiverConfigMap `json:"receivers,omitempty"`
	// ReceiverSecrets defines receiver secrets to be updated
	//
	// required: true
	ReceiverSecrets UpdateReceiverConfigSecretsMap `json:"receiverSecrets,omitempty"`

	// Alerts sets the list of alerts and mapping to a defined receiver
	//
	// required: true
	Alerts AlertConfigList `json:"alerts"`
}
