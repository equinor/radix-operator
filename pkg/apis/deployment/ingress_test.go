package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testIngressConfiguration = `
configuration:
  - name: ewma
    annotations:
      nginx.ingress.kubernetes.io/load-balance: ewma
  - name: round-robin
    annotations:
      nginx.ingress.kubernetes.io/load-balance: round_robin
  - name: socket
    annotations:
      nginx.ingress.kubernetes.io/proxy-connect-timeout: 3600
      nginx.ingress.kubernetes.io/proxy-read-timeout: 3600
      nginx.ingress.kubernetes.io/proxy-send-timeout: 3600
`

func TestGetAnnotationsFromConfigurations_ReturnsCorrectConfig(t *testing.T) {
	config := getConfigFromStringData(testIngressConfiguration)
	annotations := getAnnotationsFromConfigurations(config, "socket")
	assert.Equal(t, 3, len(annotations))

	annotations = getAnnotationsFromConfigurations(config, "socket", "round-robin")
	assert.Equal(t, 4, len(annotations))

	annotations = getAnnotationsFromConfigurations(config, "non-existing")
	assert.Equal(t, 0, len(annotations))
}
