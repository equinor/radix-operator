package resources_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/resources"
	"github.com/stretchr/testify/assert"
)

func TestMemoryResources(t *testing.T) {
	r := resources.New(resources.WithMemoryMega(1000))

	assert.Equal(t, "1G", r.Limits.Memory().String(), "Memory limit shoult be set")
	assert.Equal(t, "1G", r.Requests.Memory().String(), "Memory request shoult be set")
}

func TestCPUResources(t *testing.T) {
	r := resources.New(resources.WithCPUMilli(1000))

	assert.Equal(t, "1", r.Requests.Cpu().String(), "CPU should be set to 1 core")
	assert.Equal(t, "0", r.Limits.Cpu().String(), "CPU Limit should not be set")
}
