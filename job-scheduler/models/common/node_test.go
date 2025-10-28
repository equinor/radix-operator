package common_test

import (
	"testing"

	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_GpuNode(t *testing.T) {
	sut := models.Node{
		Gpu:      "gpu1, gpu2",
		GpuCount: "2",
	}
	expected := &radixv1.RadixNode{
		Gpu:      "gpu1, gpu2",
		GpuCount: "2",
	}
	actual := sut.MapToRadixNode()
	assert.Equal(t, expected, actual)
}
