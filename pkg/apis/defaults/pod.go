package defaults

const (
	// DefaultReplicas Hold the default replicas for the deployment if nothing is stated in the radix config
	DefaultReplicas int32 = 1
	// DefaultNodeSelectorArchitecture Hold the default architecture for the deployment if nothing is stated in the radix config
	DefaultNodeSelectorArchitecture = "amd64"
	// DefaultNodeSelectorOS Hold the default OS for the deployment if nothing is stated in the radix config
	DefaultNodeSelectorOS = "linux"
)
