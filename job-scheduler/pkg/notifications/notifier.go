package notifications

import (
	"github.com/equinor/radix-operator/job-scheduler/models/v1/events"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Notifier to notify about RadixBatch events and changes
type Notifier interface {
	// Notify Send notification
	Notify(event events.Event, radixBatch *radixv1.RadixBatch, jobStatuses []radixv1.RadixBatchJobStatus) error
	// Enabled The notifier is enabled and can be used
	Enabled() bool
	// String Describes the notifier
	String() string
}
