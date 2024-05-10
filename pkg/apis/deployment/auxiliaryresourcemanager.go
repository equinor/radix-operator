package deployment

import "context"

// AuxiliaryResourceManager contains methods to configure auxiliary resources for a deployment
type AuxiliaryResourceManager interface {
	Sync(ctx context.Context) error
	GarbageCollect(ctx context.Context) error
}
