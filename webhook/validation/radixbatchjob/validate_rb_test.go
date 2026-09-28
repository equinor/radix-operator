package radixbatchjob_test

import (
	"context"
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/webhook/validation/radixbatchjob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceRequirementsValidator(t *testing.T) {
	tests := []struct {
		name        string
		resources   *radixv1.ResourceRequirements
		expectedErr error
	}{
		{name: "resources omitted"},
		{
			name: "valid CPU and memory quantities",
			resources: &radixv1.ResourceRequirements{
				Requests: radixv1.ResourceList{"cpu": "250m", "memory": "256Mi"},
				Limits:   radixv1.ResourceList{"cpu": "1", "memory": "1Gi"},
			},
		},
		{
			name:      "CPU exactly at maximum",
			resources: &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"cpu": "1k"}},
		},
		{
			name: "request equal to limit",
			resources: &radixv1.ResourceRequirements{
				Requests: radixv1.ResourceList{"cpu": "500m", "memory": "512Mi"},
				Limits:   radixv1.ResourceList{"cpu": "500m", "memory": "512Mi"},
			},
		},
		{
			name:      "zero quantities",
			resources: &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"cpu": "0", "memory": "0"}},
		},
		{
			name:      "empty resource lists",
			resources: &radixv1.ResourceRequirements{},
		},
		{
			name:        "invalid limit syntax",
			resources:   &radixv1.ResourceRequirements{Limits: radixv1.ResourceList{"memory": "abc"}},
			expectedErr: radixbatchjob.ErrInvalidResourceFormat,
		},
		{
			name:        "invalid request syntax",
			resources:   &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"memory": "256MB"}},
			expectedErr: radixbatchjob.ErrInvalidResourceFormat,
		},
		{
			name:        "invalid limit syntax",
			resources:   &radixv1.ResourceRequirements{Limits: radixv1.ResourceList{"cpu": "one"}},
			expectedErr: radixbatchjob.ErrInvalidResourceFormat,
		},
		{
			name:        "unsupported resource type",
			resources:   &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"storage": "1Gi"}},
			expectedErr: radixbatchjob.ErrInvalidResourceType,
		},
		{
			name:        "negative quantity",
			resources:   &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"memory": "-1Mi"}},
			expectedErr: radixbatchjob.ErrNegativeResourceQuantity,
		},
		{
			name:        "CPU request above maximum",
			resources:   &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"cpu": "1001"}},
			expectedErr: radixbatchjob.ErrCPUResourceRequirementTooHigh,
		},
		{
			name:        "CPU limit above maximum",
			resources:   &radixv1.ResourceRequirements{Limits: radixv1.ResourceList{"cpu": "1001"}},
			expectedErr: radixbatchjob.ErrCPUResourceRequirementTooHigh,
		},
		{
			name: "CPU request exceeds limit",
			resources: &radixv1.ResourceRequirements{
				Requests: radixv1.ResourceList{"cpu": "500m"},
				Limits:   radixv1.ResourceList{"cpu": "250m"},
			},
			expectedErr: radixbatchjob.ErrRequestedResourceExceedsLimit,
		},
		{
			name: "memory request exceeds limit",
			resources: &radixv1.ResourceRequirements{
				Requests: radixv1.ResourceList{"memory": "1Gi"},
				Limits:   radixv1.ResourceList{"memory": "512Mi"},
			},
			expectedErr: radixbatchjob.ErrRequestedResourceExceedsLimit,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			radixBatch := &radixv1.RadixBatch{
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job-1", Resources: test.resources}},
				},
			}

			warnings, err := radixbatchjob.CreateValidator().Validate(context.Background(), radixBatch)
			assert.Empty(t, warnings)
			if test.expectedErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, test.expectedErr)
		})
	}
}

func TestResourceRequirementsValidatorAggregatesErrorsAcrossJobs(t *testing.T) {
	radixBatch := &radixv1.RadixBatch{
		Spec: radixv1.RadixBatchSpec{
			Jobs: []radixv1.RadixBatchJob{
				{Name: "job-1", Resources: &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"cpu": "invalid"}}},
				{Name: "job-2", Resources: &radixv1.ResourceRequirements{Requests: radixv1.ResourceList{"memory": "-1Mi"}}},
			},
		},
	}

	_, err := radixbatchjob.CreateValidator().Validate(context.Background(), radixBatch)
	require.Error(t, err)
	assert.ErrorIs(t, err, radixbatchjob.ErrInvalidResourceFormat)
	assert.ErrorIs(t, err, radixbatchjob.ErrNegativeResourceQuantity)
	assert.Contains(t, err.Error(), `job "job-1" at index 0`)
	assert.Contains(t, err.Error(), `job "job-2" at index 1`)
}
