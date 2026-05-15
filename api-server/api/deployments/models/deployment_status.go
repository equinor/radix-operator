package models

// DeploymentStatus Enumeration of deployment reconciliation states
// swagger:enum DeploymentStatus
type DeploymentStatus string

const (
	// DeploymentStatusReconciling deployment is not fully reconciled
	DeploymentStatusReconciling DeploymentStatus = "Reconciling"

	// DeploymentStatusReady deployment is reconciled successfully
	DeploymentStatusReady DeploymentStatus = "Ready"

	// DeploymentStatusFailed deployment reconciliation failed
	DeploymentStatusFailed DeploymentStatus = "Failed"

	// DeploymentStatusInactive deployment is inactive
	DeploymentStatusInactive DeploymentStatus = "Inactive"
)
