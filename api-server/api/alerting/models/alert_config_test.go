package models

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_AlertConfigList(t *testing.T) {
	alertConfigs := AlertConfigList{{Alert: "alert1", Receiver: "receiver1"}, {Alert: "alert2", Receiver: "receiver2"}}
	radixAlerts := alertConfigs.AsRadixAlertAlerts()
	assert.ElementsMatch(t, []v1.Alert{{Alert: "alert1", Receiver: "receiver1"}, {Alert: "alert2", Receiver: "receiver2"}}, radixAlerts)
}
