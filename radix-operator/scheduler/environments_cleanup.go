package scheduler

type environmentsCleanupTask struct {
}

// NewEnvironmentsCleanupTask Creates a new environments cleanup task
func NewEnvironmentsCleanupTask() Task {
	return &environmentsCleanupTask{}
}
