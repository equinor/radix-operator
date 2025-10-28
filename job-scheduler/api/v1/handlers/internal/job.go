package internal

import (
	"fmt"
	"strings"

	modelsv1 "github.com/equinor/radix-operator/job-scheduler/models/v1"
)

// GetBatchJobStatus Get batch job
func GetBatchJobStatus(batchStatus *modelsv1.BatchStatus, jobName string) (*modelsv1.JobStatus, error) {
	for i := range batchStatus.JobStatuses {
		if !strings.EqualFold(batchStatus.JobStatuses[i].Name, jobName) {
			continue
		}
		return &batchStatus.JobStatuses[i], nil
	}
	return nil, fmt.Errorf("not found")
}
