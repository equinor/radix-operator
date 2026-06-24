package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

func NewSyncEventRecorder(recorder record.EventRecorder) SyncEventRecorder {
	return SyncEventRecorder{Recorder: recorder}
}

type SyncEventRecorder struct {
	Recorder record.EventRecorder
}

func (r SyncEventRecorder) RecordSyncSuccessEvent(obj runtime.Object) {
	r.Recorder.Event(obj, corev1.EventTypeNormal, "Synced", "Successfully synced")
}

func (r SyncEventRecorder) RecordSyncErrorEvent(obj runtime.Object, err error) {
	r.Recorder.Eventf(obj, corev1.EventTypeWarning, "SyncFailed", "Failed to sync: %s", err.Error())
}
