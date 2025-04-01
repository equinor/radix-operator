package event

import (
	radixscheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// NewRecorder Creates an event recorder for controller
func NewRecorder(controllerAgentName string, events typedcorev1.EventInterface, logger zerolog.Logger) record.EventRecorder {
	if err := radixscheme.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	logger.Info().Msg("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	// TODO: Should we skip setting StartLogging? This generates many duplicate records in the log
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) { logger.Info().Msgf(format, args...) })
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
}
