package scheduledjob

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	invalidDeploymentReferenceReason = "InvalidDeploymentReference"
)

func newReconcileWaitingError(reason, message string) *reconcileError {
	return &reconcileError{
		status: radixv1.RadixScheduledJobStatus{
			Phase:   radixv1.ScheduledJobPhaseWaiting,
			Reason:  reason,
			Message: message,
		},
	}
}

func newReconcileRadixDeploymentNotFoundError(rdName string) *reconcileError {
	return newReconcileWaitingError(invalidDeploymentReferenceReason, fmt.Sprintf("radixdeployment '%s' not found", rdName))
}

func newReconcileRadixDeploymentJobSpecNotFoundError(rdName, jobName string) *reconcileError {
	return newReconcileWaitingError(invalidDeploymentReferenceReason, fmt.Sprintf("radixdeployment '%s' does not contain a job with name '%s'", rdName, jobName))
}

type reconcileError struct {
	status radixv1.RadixScheduledJobStatus
}

func (rc *reconcileError) Error() string {
	return rc.status.Message
}

func (rc *reconcileError) Status() radixv1.RadixScheduledJobStatus {
	return rc.status
}

type reconcileStatus interface {
	Status() radixv1.RadixScheduledJobStatus
}
