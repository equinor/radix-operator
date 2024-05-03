package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixAlert describe alert config for a radix environment
type RadixAlert struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixAlertSpec   `json:"spec" yaml:"spec"`
	Status             RadixAlertStatus `json:"status" yaml:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixAlertList is a list of RadixAlert
type RadixAlertList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixAlert `json:"items" yaml:"items"`
}

// RadixAlertSpec is the spec for a RadixAlert
type RadixAlertSpec struct {
	Receivers ReceiverMap `json:"receivers" yaml:"receivers"`
	Alerts    []Alert     `json:"alerts" yaml:"alerts"`
}

// RadixAlertStatus is the status for a RadixAlert
type RadixAlertStatus struct {
	Reconciled *meta_v1.Time `json:"reconciled" yaml:"reconciled"`
}

type ReceiverMap map[string]Receiver

type Receiver struct {
	SlackConfig SlackConfig `json:"slackConfig" yaml:"slackConfig"`
}

func (receiver *Receiver) IsEnabled() bool {
	if receiver == nil {
		return false
	}

	return receiver.SlackConfig.Enabled
}

type SlackConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type Alert struct {
	Alert    string `json:"alert" yaml:"alert"`
	Receiver string `json:"receiver" yaml:"receiver"`
}
