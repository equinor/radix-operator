package event

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// NewRecorder Creates an event recorder for controller
func NewRecorder(controllerAgentName string, events typedcorev1.EventInterface) (record.EventRecorder, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: controllerAgentName}), nil
}
