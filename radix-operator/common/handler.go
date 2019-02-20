package common

import "k8s.io/client-go/tools/record"

type Handler interface {
	Sync(namespace, name string, eventRecorder record.EventRecorder) error
}
