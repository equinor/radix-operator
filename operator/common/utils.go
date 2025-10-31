package common

import (
	"context"
	"sync"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/tools/cache"
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

// LockKeyFunc is a function which is either NamespacePartitionKey or NamePartitionKey
type LockKeyFunc func(obj cache.ObjectName) (lockKey string)

// NamespacePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixApplications,
// RadixAlerts, RadixDeployments and RadixJobs
func NamespacePartitionKey(obj cache.ObjectName) (lockKey string) {
	return obj.Namespace
}

// NamePartitionKey returns a string which serves as a "locking key" for parallel processing of RadixEnvironments and
// RadixRegistrations
func NamePartitionKey(obj cache.ObjectName) (lockKey string) {
	return obj.Name
}

// GetOwner Function pointer to pass to retrieve owner
type GetOwner func(context.Context, radixclient.Interface, string, string) (interface{}, error)

func NewRateLimitedWorkQueue(ctx context.Context, name string) workqueue.TypedRateLimitingInterface[cache.ObjectName] {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[cache.ObjectName](), workqueue.TypedRateLimitingQueueConfig[cache.ObjectName]{Name: name})
	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()

	return queue
}
