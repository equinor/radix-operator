package models

// UpdateReceiverConfigSecretsMap defines a map of UpdateReceiverConfigSecrets where key is the name of the receiver
type UpdateReceiverConfigSecretsMap map[string]UpdateReceiverConfigSecrets

// UpdateReceiverConfigSecrets defines secrets to be updated
type UpdateReceiverConfigSecrets struct {
	// SlackConfig defines Slack secrets to update for this receiver
	// Secrets will be updated if slackConfig is non-nil
	SlackConfig *UpdateSlackConfigSecrets `json:"slackConfig"`
}

// UpdateSlackConfig defines secrets to be updated for Slack
type UpdateSlackConfigSecrets struct {
	// WebhookURL the Slack webhook URL where alerts are sent
	// Secret key for webhook URL is updated if a non-nil value is present, and deleted if omitted or set to null
	//
	// required:
	// Extensions:
	// x-nullable: true
	WebhookURL *string `json:"webhookUrl,omitempty"`
}
