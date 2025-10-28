package common_test

import (
	"testing"

	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_MapToRadixResourceRequirements(t *testing.T) {
	sut := models.Resources{
		Limits: models.ResourceList{
			"cpu":    "10m",
			"memory": "10M",
		},
		Requests: models.ResourceList{
			"cpu":    "20m",
			"memory": "20M",
		},
	}
	expected := &radixv1.ResourceRequirements{
		Limits: radixv1.ResourceList{
			"cpu":    "10m",
			"memory": "10M",
		},
		Requests: radixv1.ResourceList{
			"cpu":    "20m",
			"memory": "20M",
		},
	}
	actual := sut.MapToRadixResourceRequirements()
	assert.Equal(t, expected, actual)
}
