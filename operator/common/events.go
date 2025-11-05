package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type SyncedEventRecorder struct {
	EventRecorder record.EventRecorder
}

func (r SyncedEventRecorder) RecordSyncSuccessEvent(obj runtime.Object) {
	r.EventRecorder.Event(obj, corev1.EventTypeNormal, "Synced", "Successfully synced")
}

func (r SyncedEventRecorder) RecordSyncErrorEvent(obj runtime.Object, err error) {
	r.EventRecorder.Eventf(obj, corev1.EventTypeWarning, "SyncFailed", "Failed to sync: %s", err.Error())
}
