package scheduledjob

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

func newReconcileWaitingError(reason, message string) *reconcileError {
	return &reconcileError{
		status: radixv1.RadixScheduledJobStatus{
			Phase:   radixv1.ScheduledJobPhaseWaiting,
			Reason:  reason,
			Message: message,
		},
	}
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
