package models

// BuildSecretStatus Enumeration of the statuses of a build secret
type BuildSecretStatus int

const (
	// Pending In configuration but not in cluster
	Pending BuildSecretStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	numStatuses
)

func (p BuildSecretStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent"}[p]
}
