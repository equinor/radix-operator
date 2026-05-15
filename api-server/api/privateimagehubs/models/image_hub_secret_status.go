package models

// ImageHubSecretStatus Enumeration of the statuses of an image hub secret
type ImageHubSecretStatus int

const (
	// Pending In configuration but not in cluster
	Pending ImageHubSecretStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	numStatuses
)

func (p ImageHubSecretStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent"}[p]
}
