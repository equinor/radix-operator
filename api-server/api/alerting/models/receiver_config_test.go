package models

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_ReceiverConfigMap(t *testing.T) {
	receiverConfig := ReceiverConfigMap{
		"receiver1": ReceiverConfig{SlackConfig: &SlackConfig{Enabled: true}},
		"receiver2": ReceiverConfig{SlackConfig: &SlackConfig{Enabled: false}},
	}

	radixReceivers := receiverConfig.AsRadixAlertReceiverMap()
	assert.Len(t, radixReceivers, 2)
	assert.Equal(t, v1.Receiver{SlackConfig: v1.SlackConfig{Enabled: true}}, radixReceivers["receiver1"])
	assert.Equal(t, v1.Receiver{SlackConfig: v1.SlackConfig{Enabled: false}}, radixReceivers["receiver2"])
}
