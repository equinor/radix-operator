package batch

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	invalidDeploymentReferenceReason = "InvalidDeploymentReference"
)

func newReconcileWaitingError(reason, message string) *reconcileError {
	return &reconcileError{
		status: radixv1.RadixBatchCondition{
			Type:    radixv1.BatchConditionTypeWaiting,
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
	status radixv1.RadixBatchCondition
}

func (rc *reconcileError) Error() string {
	return rc.status.Message
}

func (rc *reconcileError) Status() radixv1.RadixBatchCondition {
	return rc.status
}

type reconcileStatus interface {
	Status() radixv1.RadixBatchCondition
}
