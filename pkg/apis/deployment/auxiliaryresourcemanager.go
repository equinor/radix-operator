package deployment

// AuxiliaryResourceManager contains methods to configure auxiliary resources for a deployment
type AuxiliaryResourceManager interface {
	Sync() error
	GarbageCollect() error
}
