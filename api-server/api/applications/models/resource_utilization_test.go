package models_test

import (
	"testing"

	"github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentUtilization(t *testing.T) {
	r := models.NewPodResourcesUtilizationResponse()

	assert.Empty(t, r.Environments)

	r.SetCpuRequests("dev", "web", "web-abccdc-1234", 1)
	r.SetMemoryRequests("prod", "srv", "srv-abccdc-1234", 2)
	r.SetMemoryMaximum("dev", "web", "web-abccdc-1234", 1500)
	r.SetCpuAverage("prod", "srv", "srv-abccdc-1234", 2.5)

	require.Len(t, r.Environments, 2)
	require.Contains(t, r.Environments, "dev")
	require.Contains(t, r.Environments, "prod")
	require.Len(t, r.Environments["dev"].Components, 1)
	require.Len(t, r.Environments["prod"].Components, 1)
	require.Len(t, r.Environments["dev"].Components["web"].Replicas, 1)
	require.Len(t, r.Environments["prod"].Components["srv"].Replicas, 1)

	require.Contains(t, r.Environments["dev"].Components["web"].Replicas, "web-abccdc-1234")
	require.Contains(t, r.Environments["prod"].Components["srv"].Replicas, "srv-abccdc-1234")

	assert.Equal(t, 1.0, r.Environments["dev"].Components["web"].Replicas["web-abccdc-1234"].CpuRequests)
	assert.Equal(t, 2.0, r.Environments["prod"].Components["srv"].Replicas["srv-abccdc-1234"].MemoryRequests)

	assert.Equal(t, 1500.0, r.Environments["dev"].Components["web"].Replicas["web-abccdc-1234"].MemoryMaximum)
	assert.Equal(t, 2.5, r.Environments["prod"].Components["srv"].Replicas["srv-abccdc-1234"].CpuAverage)
}
