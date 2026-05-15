package models

// ContainerStatus Enumeration of the statuses of container
// swagger:enum ContainerStatus
type ContainerStatus string

const (
	// Pending container
	Pending ContainerStatus = "Pending"

	// Failed container permanently exists in a failed state
	Failed ContainerStatus = "Failed"

	// Failing container, which can attempt to restart
	Failing ContainerStatus = "Failing"

	// Running container
	Running ContainerStatus = "Running"

	// Terminated container
	Terminated ContainerStatus = "Terminated"

	// Starting container
	Starting ContainerStatus = "Starting"

	// Stopped container
	Stopped ContainerStatus = "Stopped"

	// Succeeded all containers in the pod have voluntarily terminated
	Succeeded ContainerStatus = "Succeeded"
)
