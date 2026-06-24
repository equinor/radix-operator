package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Event_Marshal(t *testing.T) {
	event := Event{
		LastTimestamp:           time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		InvolvedObjectKind:      "akind",
		InvolvedObjectNamespace: "anamespace",
		InvolvedObjectName:      "aname",
		Type:                    "atype",
		Reason:                  "areason",
		Message:                 "amessage",
	}
	expected := "{\"lastTimestamp\":\"2020-01-02T03:04:05Z\",\"involvedObjectKind\":\"akind\",\"involvedObjectNamespace\":\"anamespace\",\"involvedObjectName\":\"aname\",\"type\":\"atype\",\"reason\":\"areason\",\"message\":\"amessage\"}"
	eventbytes, err := json.Marshal(event)
	assert.Nil(t, err)
	eventjson := string(eventbytes)
	assert.Equal(t, expected, eventjson)
}
