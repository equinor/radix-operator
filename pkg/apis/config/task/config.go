package task

import "time"

// Config for tasks
type Config struct {
	// OrphanedRadixEnvironmentsRetentionPeriod is the time period for how long orphaned RadixEnvironments should be retained
	OrphanedRadixEnvironmentsRetentionPeriod time.Duration
	// OrphanedEnvironmentsCleanupCron is the cron expression for when to run the cleanup of orphaned RadixEnvironments
	OrphanedEnvironmentsCleanupCron string
}
