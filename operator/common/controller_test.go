package common

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/cache"
)

type mockResourceLocker struct {
	mock.Mock
}

func (m *mockResourceLocker) TryGetLock(key string) bool { return m.Called(key).Bool(0) }
func (m *mockResourceLocker) ReleaseLock(key string)     { m.Called(key) }

type mockRateLimitingQueue struct {
	getCh      chan cache.ObjectName
	shutdownCh chan bool
	mock.Mock
}

func (m *mockRateLimitingQueue) AddRateLimited(item cache.ObjectName) { m.Called(item) }
func (m *mockRateLimitingQueue) Forget(item cache.ObjectName)         { m.Called(item) }
func (m *mockRateLimitingQueue) NumRequeues(item cache.ObjectName) int {
	return m.Called(item).Int(0)
}
func (m *mockRateLimitingQueue) AddAfter(item cache.ObjectName, duration time.Duration) {
	m.Called(item, duration)
}
func (m *mockRateLimitingQueue) Add(item cache.ObjectName) { m.Called(item) }
func (m *mockRateLimitingQueue) Len() int {
	return m.Called().Int(0)
}
func (m *mockRateLimitingQueue) Get() (item cache.ObjectName, shutdown bool) {
	return <-m.getCh, <-m.shutdownCh
}
func (m *mockRateLimitingQueue) Done(item cache.ObjectName) { m.Called(item) }
func (m *mockRateLimitingQueue) ShutDown()                  { m.Called() }
func (m *mockRateLimitingQueue) ShutDownWithDrain()         { m.Called() }
func (m *mockRateLimitingQueue) ShuttingDown() bool         { return m.Called().Bool(0) }

type commonControllerTestSuite struct {
	ControllerTestSuite
}

func TestCommonControllerTestSuite(t *testing.T) {
	suite.Run(t, new(commonControllerTestSuite))
}

func (s *commonControllerTestSuite) Test_SyncSuccess() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return obj.Namespace
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(stopCh)

	go func() {
		err := sut.Run(context.Background(), 1)
		s.Require().NoError(err)
	}()

	doneCh := make(chan struct{})
	item := cache.NewObjectName("ns", "item")
	locker.On("TryGetLock", "ns").Return(true).Times(1)
	locker.On("ReleaseLock", "ns").Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("Forget", item).Times(1)
	queue.On("Done", item).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	s.Handler.EXPECT().Sync(gomock.Any(), "ns", "item").Return(nil).Times(1)
	queue.getCh <- item
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_RequeueWhenSyncError() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return obj.Namespace
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(stopCh)

	go func() {
		err := sut.Run(context.Background(), 1)
		s.Require().NoError(err)
	}()

	doneCh := make(chan struct{})
	item := cache.NewObjectName("ns", "item")
	locker.On("TryGetLock", "ns").Return(true).Times(1)
	locker.On("ReleaseLock", "ns").Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("AddRateLimited", item).Times(1)
	queue.On("Done", item).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	s.Handler.EXPECT().Sync(gomock.Any(), "ns", "item").Return(errors.New("any error")).Times(1)
	queue.getCh <- item
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_QuitRunWhenShutdownTrue() {
	stopCh := make(chan struct{})

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return "any"
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(stopCh)

	doneCh := make(chan struct{})
	go func() {
		err := sut.Run(context.Background(), 1)
		s.Require().NoError(err)
		close(doneCh)
	}()

	queue.getCh <- cache.NewObjectName("ns", "item")
	queue.shutdownCh <- true

	select {
	case <-doneCh:
	case <-time.NewTimer(5 * time.Second).C:
		s.FailNow("controller did not honor shutdown from workqueue.Get()")
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_QuitRunWhenShuttingDownTrue() {
	stopCh := make(chan struct{})

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return "any"
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(stopCh)

	doneCh := make(chan struct{})
	go func() {
		err := sut.Run(context.Background(), 1)
		close(doneCh)
		s.Require().NoError(err)
	}()

	queue.On("ShuttingDown").Return(true).Times(1)
	queue.getCh <- cache.NewObjectName("ns", "item")
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
		s.FailNow("controller did not honor shutdown from workqueue.Get()")
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_RequeueWhenLocked() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return obj.Namespace
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(stopCh)

	go func() {
		err := sut.Run(context.Background(), 1)
		s.Require().NoError(err)
	}()

	doneCh := make(chan struct{})
	item := cache.NewObjectName("ns", "item")
	locker.On("TryGetLock", "ns").Return(false).Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("AddAfter", item, 100*time.Millisecond).Times(1)
	queue.On("Done", item).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	queue.getCh <- item
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_ProcessParallell() {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	queue := &mockRateLimitingQueue{getCh: make(chan cache.ObjectName, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		RadixClient: s.RadixClient,
		LockKey: func(obj cache.ObjectName) (lockKey string) {
			return obj.Namespace
		},
		WorkQueue:            queue,
		locker:               locker,
		KubeInformerFactory:  s.KubeInformerFactory,
		RadixInformerFactory: s.RadixInformerFactory,
	}

	s.KubeInformerFactory.Start(ctx.Done())

	// Test that threadiness limit is used and not exceeded
	doneCh := make(chan struct{})
	maxThreadsCh := make(chan int32, 1)
	maxThreadsCh <- 0
	defer close(maxThreadsCh)
	testItems := make([]cache.ObjectName, 0, 100)
	for i := 0; i < 100; i++ {
		testItems = append(testItems, cache.NewObjectName("ns", strconv.Itoa(i)))
	}
	var active, iteration int32
	threadiness := 5
	queue.On("ShuttingDown").Return(false)

	go func() {
		err := sut.Run(ctx, threadiness)
		s.Require().NoError(err)
	}()

	for i := 0; i < len(testItems); i++ {
		i := i
		item := testItems[i]

		locker.On("TryGetLock", item.Namespace).Return(true).Times(1)
		locker.On("ReleaseLock", item.Namespace).Times(1)
		queue.On("Done", item).Times(1).Run(func(args mock.Arguments) {
			n := atomic.AddInt32(&iteration, 1)
			// Close doneCh when Done is called for the last item
			if int(n) == len(testItems) {
				close(doneCh)
			}
		})
		queue.On("Forget", item).Times(1)
		s.Handler.EXPECT().Sync(gomock.Any(), item.Namespace, item.Name).Times(1).DoAndReturn(func(ctx context.Context, namespace, name string) error {
			n := atomic.AddInt32(&active, 1)

			// Set new number of active threads if it exceeds previous value
			t := <-maxThreadsCh
			if n > t {
				t = n
			}
			maxThreadsCh <- t

			time.Sleep(1 * time.Millisecond) // Sleep to give other goroutines a chance to increment active.
			atomic.AddInt32(&active, -1)
			return nil
		})

		queue.getCh <- item
		queue.shutdownCh <- false
	}

	select {
	case <-doneCh:
		// Check if max number of goroutines didn't exceed threadiness
		actualMax := <-maxThreadsCh
		expectedMax := threadiness
		if len(testItems) < threadiness {
			expectedMax = len(testItems)
		}
		s.Equal(int(actualMax), expectedMax)
	case <-time.NewTimer(5 * time.Second).C:
		s.FailNow("timeout waiting for controller to process items")
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}
