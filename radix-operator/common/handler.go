package common

type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(key string)
	ObjectUpdated(objOld, objNew interface{})
}
