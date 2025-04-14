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

func NewRateLimitedWorkQueue(ctx context.Context, name string) workqueue.TypedRateLimitingInterface[string] {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](), workqueue.TypedRateLimitingQueueConfig[string]{Name: name})
	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()

	return queue
}
