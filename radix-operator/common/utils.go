package common

import (
	"fmt"
	"sync"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixscheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

type resourceLocker interface {
	TryGetLock(key string) bool
	ReleaseLock(key string)
}

type defaultResourceLocker struct {
	l sync.Map
}

func (r *defaultResourceLocker) TryGetLock(key string) bool {
	_, loaded := r.l.LoadOrStore(key, true)

	return !loaded
}

func (r *defaultResourceLocker) ReleaseLock(key string) {
	r.l.Delete(key)
}

// LockKeyAndIdentifierFunc is a function which is either NamespacePartitionKey or NamePartitionKey
type LockKeyAndIdentifierFunc func(obj interface{}) (lockKey string, identifier string, err error)

func getStringFromObj(obj interface{}) (string, error) {
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		return "", fmt.Errorf("expected string in workqueue but got %#v", obj)
	}
	return key, nil
}

// NamespacePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixApplications,
// RadixAlerts, RadixDeployments and RadixJobs
func NamespacePartitionKey(obj interface{}) (lockKey string, identifier string, err error) {
	identifier, err = getStringFromObj(obj)
	if err != nil {
		return
	}
	lockKey, _, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

// NamePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixEnvironments and
// RadixRegistrations
func NamePartitionKey(obj interface{}) (lockKey string, identifier string, err error) {
	identifier, err = getStringFromObj(obj)
	if err != nil {
		return
	}
	_, lockKey, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

// GetOwner Function pointer to pass to retrieve owner
type GetOwner func(radixclient.Interface, string, string) (interface{}, error)

// NewEventRecorder Creates an event recorder for controller
func NewEventRecorder(controllerAgentName string, events typedcorev1.EventInterface, logger zerolog.Logger) record.EventRecorder {
	if err := radixscheme.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	logger.Info().Msg("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) { logger.Info().Msgf(format, args...) })
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
}
