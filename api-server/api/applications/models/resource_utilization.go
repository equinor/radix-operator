package models

// ReplicaResourcesUtilizationResponse holds information about resource utilization
// swagger:model ReplicaResourcesUtilizationResponse
type ReplicaResourcesUtilizationResponse struct {
	Environments map[string]EnvironmentUtilization `json:"environments"`
}

type EnvironmentUtilization struct {
	Components map[string]ComponentUtilization `json:"components"`
}

type ComponentUtilization struct {
	Replicas map[string]ReplicaUtilization `json:"replicas"`
}
type ReplicaUtilization struct {
	// Memory Requests
	// required: true
	MemoryRequests float64 `json:"memoryRequests"`
	// Max memory used
	// required: true
	MemoryMaximum float64 `json:"memoryMaximum"`
	// Cpu Requests
	// required: true
	CpuRequests float64 `json:"cpuRequests"`
	// Average CPU Used
	// required: true
	CpuAverage float64 `json:"cpuAverage"`
}

func NewPodResourcesUtilizationResponse() *ReplicaResourcesUtilizationResponse {
	return &ReplicaResourcesUtilizationResponse{
		Environments: make(map[string]EnvironmentUtilization),
	}
}

func (r *ReplicaResourcesUtilizationResponse) SetCpuRequests(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.CpuRequests = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetCpuAverage(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.CpuAverage = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetMemoryRequests(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.MemoryRequests = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetMemoryMaximum(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.MemoryMaximum = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) ensurePod(environment, component, pod string) {
	if _, ok := r.Environments[environment]; !ok {
		r.Environments[environment] = EnvironmentUtilization{
			Components: make(map[string]ComponentUtilization),
		}
	}

	if _, ok := r.Environments[environment].Components[component]; !ok {
		r.Environments[environment].Components[component] = ComponentUtilization{
			Replicas: make(map[string]ReplicaUtilization),
		}
	}

	if _, ok := r.Environments[environment].Components[component].Replicas[pod]; !ok {
		r.Environments[environment].Components[component].Replicas[pod] = ReplicaUtilization{}
	}
}
