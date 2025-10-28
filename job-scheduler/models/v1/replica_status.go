package v1

// ReplicaStatus describes the status of a component container inside a pod
// swagger:model ReplicaStatus
type ReplicaStatus struct {
	// Status of the container
	// - Pending = Container in Waiting state and the reason is ContainerCreating
	// - Failed = Container is failed
	// - Failing = Container is failed
	// - Running = Container in Running state
	// - Succeeded = Container in Succeeded state
	// - Terminated = Container in Terminated state
	// - Stopped = Job has been stopped
	//
	// required: true
	// enum: Pending,Succeeded,Failing,Failed,Running,Terminated,Starting,Stopped
	// example: Running
	Status string `json:"status"`
}
