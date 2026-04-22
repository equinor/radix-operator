package job

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
)

func Test_findJobCondition(t *testing.T) {
	tests := []struct {
		name      string
		radixJob  *radixv1.RadixJob
		jobStatus batchv1.JobStatus
		expected  radixv1.RadixJobCondition
	}{
		{
			name:      "failed job returns JobFailed",
			jobStatus: batchv1.JobStatus{Failed: 1},
			expected:  radixv1.JobFailed,
		},
		{
			name:      "failed takes precedence over active",
			jobStatus: batchv1.JobStatus{Failed: 1, Active: 1},
			expected:  radixv1.JobFailed,
		},
		{
			name:      "failed takes precedence over succeeded",
			jobStatus: batchv1.JobStatus{Failed: 1, Succeeded: 1},
			expected:  radixv1.JobFailed,
		},
		{
			name:      "active job returns JobRunning",
			jobStatus: batchv1.JobStatus{Active: 1},
			expected:  radixv1.JobRunning,
		},
		{
			name:      "active takes precedence over succeeded",
			jobStatus: batchv1.JobStatus{Active: 1, Succeeded: 1},
			expected:  radixv1.JobRunning,
		},
		{
			name:      "succeeded job returns JobSucceeded",
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobSucceeded,
		},
		{
			name:      "succeeded with nil radixJob returns JobSucceeded",
			radixJob:  nil,
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobSucceeded,
		},
		{
			name: "succeeded with nil PipelineRunStatus returns JobSucceeded",
			radixJob: &radixv1.RadixJob{
				Status: radixv1.RadixJobStatus{PipelineRunStatus: nil},
			},
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobSucceeded,
		},
		{
			name: "succeeded with empty PipelineRunStatus.Status returns JobSucceeded",
			radixJob: &radixv1.RadixJob{
				Status: radixv1.RadixJobStatus{
					PipelineRunStatus: &radixv1.RadixJobPipelineRunStatus{Status: ""},
				},
			},
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobSucceeded,
		},
		{
			name: "succeeded with PipelineRunStatus.Status returns that status",
			radixJob: &radixv1.RadixJob{
				Status: radixv1.RadixJobStatus{
					PipelineRunStatus: &radixv1.RadixJobPipelineRunStatus{Status: radixv1.JobStoppedNoChanges},
				},
			},
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobStoppedNoChanges,
		},
		{
			name: "succeeded with PipelineRunStatus.Status Failed returns JobFailed from pipeline",
			radixJob: &radixv1.RadixJob{
				Status: radixv1.RadixJobStatus{
					PipelineRunStatus: &radixv1.RadixJobPipelineRunStatus{Status: radixv1.JobFailed},
				},
			},
			jobStatus: batchv1.JobStatus{Succeeded: 1},
			expected:  radixv1.JobFailed,
		},
		{
			name:      "zero counts returns JobWaiting",
			jobStatus: batchv1.JobStatus{},
			expected:  radixv1.JobWaiting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &Job{radixJob: tt.radixJob}
			actual := j.findJobCondition(tt.jobStatus)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
