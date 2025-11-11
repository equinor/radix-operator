package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=radixalerts,shortName=ral
// +kubebuilder:subresource:status

// RadixAlert describes configuration for setting up alerts for environments or pipeline jobs
type RadixAlert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec is the desired state of the RadixAlert
	Spec RadixAlertSpec `json:"spec"`
	// Status is the observed state of the RadixAlert
	// +kubebuilder:validation:Optional
	Status RadixAlertStatus `json:"status,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixAlertList is a list of RadixAlert
type RadixAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RadixAlert `json:"items"`
}

// RadixAlertSpec is the spec for a RadixAlert
type RadixAlertSpec struct {
	Receivers ReceiverMap `json:"receivers"`
	Alerts    []Alert     `json:"alerts"`
}

type RadixAlertReconcileStatus string

const (
	RadixAlertReconcileSucceeded RadixAlertReconcileStatus = "Succeeded"
	RadixAlertReconcileFailed    RadixAlertReconcileStatus = "Failed"
)

// RadixAlertStatus is the observed state of the RadixAlert
type RadixAlertStatus struct {
	// Reconciled is the timestamp of the last successful reconciliation
	// +kubebuilder:validation:Optional
	Reconciled metav1.Time `json:"reconciled,omitzero"`
	// ObservedGeneration is the generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ReconcileStatus indicates whether the last reconciliation succeeded or failed
	// +kubebuilder:validation:Optional
	ReconcileStatus RadixAlertReconcileStatus `json:"reconcileStatus,omitempty"`
	// Message provides additional information about the reconciliation state, typically error details when reconciliation fails
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
}

type ReceiverMap map[string]Receiver

// Receiver defines the configuration for an alert receiver
type Receiver struct {
	// SlackConfig contains the Slack-specific configuration for this receiver
	SlackConfig SlackConfig `json:"slackConfig"`
}

// SlackConfig defines the Slack configuration for an alert receiver
type SlackConfig struct {
	// Enabled indicates whether Slack notifications are enabled for this receiver
	Enabled bool `json:"enabled"`
}

// Alert defines the mapping between an alert name and its receiver
type Alert struct {
	// Alert is the name of the alert to configure
	// +kubebuilder:validation:Required
	Alert string `json:"alert"`
	// Receiver is the name of the receiver that should handle this alert
	// +kubebuilder:validation:Required
	Receiver string `json:"receiver"`
}

func (receiver *Receiver) IsEnabled() bool {
	if receiver == nil {
		return false
	}

	return receiver.SlackConfig.Enabled
}
