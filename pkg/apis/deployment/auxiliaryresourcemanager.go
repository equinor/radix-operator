package deployment

import "context"

// AuxiliaryResourceManager contains methods to configure auxiliary resources for a deployment
type AuxiliaryResourceManager interface {
	// Sync synchronizes auxiliary resources for a deployment
	Sync(ctx context.Context) error
	// GarbageCollect returns the auxiliary resources for a deployment
	GarbageCollect(ctx context.Context) error
}
