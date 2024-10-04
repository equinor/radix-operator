package tasks

// Task Interface for tasks
type Task interface {
	// Run Runs the task
	Run()
	// String Returns the task description
	String() string
}
