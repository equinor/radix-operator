package common

type Handler interface {
	Init() error
	ObjectCreated(obj interface{}) error
	ObjectDeleted(key string) error
	ObjectUpdated(objOld, objNew interface{}) error
}
