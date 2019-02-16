package common

import "k8s.io/client-go/tools/record"

type Handler interface {
	ObjectCreated(obj interface{}) error
	ObjectDeleted(key string) error
	ObjectUpdated(objOld, objNew interface{}) error
	Sync(namespace, name string, eventRecorder record.EventRecorder) error
}
