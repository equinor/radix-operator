package common

import (
	"context"
	"sync"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixscheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
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
type LockKeyAndIdentifierFunc func(obj string) (lockKey string, identifier string, err error)

// NamespacePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixApplications,
// RadixAlerts, RadixDeployments and RadixJobs
func NamespacePartitionKey(obj string) (lockKey string, identifier string, err error) {
	identifier = obj
	lockKey, _, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

// NamePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixEnvironments and
// RadixRegistrations
func NamePartitionKey(obj string) (lockKey string, identifier string, err error) {
	identifier = obj
	_, lockKey, err = cache.SplitMetaNamespaceKey(identifier)
	return
}

// GetOwner Function pointer to pass to retrieve owner
type GetOwner func(context.Context, radixclient.Interface, string, string) (interface{}, error)

// NewEventRecorder Creates an event recorder for controller
func NewEventRecorder(controllerAgentName string, events typedcorev1.EventInterface, logger zerolog.Logger) record.EventRecorder {
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

func NewRateLimitedWorkQueue(ctx context.Context, name string) workqueue.TypedRateLimitingInterface[string] {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: name})
	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()

	return queue
}
