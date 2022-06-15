package common

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/tools/record"
)

type mockResourceLocker struct {
	mock.Mock
}

func (m *mockResourceLocker) TryGetLock(key string) bool { return m.Called(key).Bool(0) }
func (m *mockResourceLocker) ReleaseLock(key string)     { m.Called(key) }

type mockRateLimitingQueue struct {
	getCh      chan interface{}
	shutdownCh chan bool
	mock.Mock
}

func (m *mockRateLimitingQueue) AddRateLimited(item interface{}) { m.Called(item) }
func (m *mockRateLimitingQueue) Forget(item interface{})         { m.Called(item) }
func (m *mockRateLimitingQueue) NumRequeues(item interface{}) int {
	args := m.Called(item)
	return args.Int(0)
}
func (m *mockRateLimitingQueue) AddAfter(item interface{}, duration time.Duration) {}
func (m *mockRateLimitingQueue) Add(item interface{})                              {}
func (m *mockRateLimitingQueue) Len() int                                          { return 0 }
func (m *mockRateLimitingQueue) Get() (item interface{}, shutdown bool) {
	return <-m.getCh, <-m.shutdownCh
}
func (m *mockRateLimitingQueue) Done(item interface{}) { m.Called(item) }
func (m *mockRateLimitingQueue) ShutDown()             {}
func (m *mockRateLimitingQueue) ShutDownWithDrain()    {}
func (m *mockRateLimitingQueue) ShuttingDown() bool    { return m.Called().Bool(0) }

type commonControllerTestSuite struct {
	ControllerTestSuite
}

func TestCommonControllerTestSuite(t *testing.T) {
	suite.Run(t, new(commonControllerTestSuite))
}

func (s *commonControllerTestSuite) Test_SyncSuccess() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			parts := strings.Split(obj.(string), "/")
			return parts[0], obj.(string), nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	item := "ns/item"
	locker.On("TryGetLock", "ns").Return(true).Times(1)
	locker.On("ReleaseLock", "ns").Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("Forget", item).Times(1)
	queue.On("Done", item).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	s.Handler.EXPECT().Sync("ns", "item", gomock.Any()).Return(nil).Times(1)
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

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			parts := strings.Split(obj.(string), "/")
			return parts[0], obj.(string), nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	item := "ns/item"
	locker.On("TryGetLock", "ns").Return(true).Times(1)
	locker.On("ReleaseLock", "ns").Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("AddRateLimited", item).Times(1)
	queue.On("Done", item).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	s.Handler.EXPECT().Sync("ns", "item", gomock.Any()).Return(errors.New("any error")).Times(1)
	queue.getCh <- item
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_ForgetWhenLockKeyAndIdentifierError() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			return "", "", errors.New("any error")
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	item := "ns/item"
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("Forget", item).Times(1)
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

func (s *commonControllerTestSuite) Test_SkipItemWhenNil() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			return "any", "any", nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("Done", nil).Times(1).Run(func(args mock.Arguments) { close(doneCh) })
	queue.getCh <- nil
	queue.shutdownCh <- false

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_SkipItemWhenEmpty() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			return "any", "any", nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	item := ""
	queue.On("ShuttingDown").Return(false).Times(1)
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

func (s *commonControllerTestSuite) Test_QuitRunWhenShutdownTrue() {
	stopCh := make(chan struct{})

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			return "any", "any", nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	doneCh := make(chan struct{})
	go func() {
		sut.Run(1, stopCh)
		close(doneCh)
	}()

	queue.getCh <- nil
	queue.shutdownCh <- true

	select {
	case <-doneCh:
	case <-time.NewTimer(time.Second).C:
		s.FailNow("controller did not honor shutdown from workqueue.Get()")
	}

	locker.AssertExpectations(s.T())
	queue.AssertExpectations(s.T())
}

func (s *commonControllerTestSuite) Test_QuitRunWhenShuttingDownTrue() {
	stopCh := make(chan struct{})

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			return "any", "any", nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	doneCh := make(chan struct{})
	go func() {
		sut.Run(1, stopCh)
		close(doneCh)
	}()

	queue.On("ShuttingDown").Return(true).Times(1)
	queue.getCh <- "any"
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

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			parts := strings.Split(obj.(string), "/")
			return parts[0], obj.(string), nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	go sut.Run(1, stopCh)

	doneCh := make(chan struct{})
	item := "ns/item"
	locker.On("TryGetLock", "ns").Return(false).Times(1)
	queue.On("ShuttingDown").Return(false).Times(1)
	queue.On("AddRateLimited", item).Times(1)
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
	stopCh := make(chan struct{})
	defer close(stopCh)

	queue := &mockRateLimitingQueue{getCh: make(chan interface{}, 1), shutdownCh: make(chan bool, 1)}
	locker := &mockResourceLocker{}
	sut := &Controller{
		Handler:     s.Handler,
		Log:         log.NewEntry(log.StandardLogger()),
		RadixClient: s.RadixClient,
		Informer:    s.KubeInformerFactory.Core().V1().ConfigMaps().Informer(),
		LockKeyAndIdentifier: func(obj interface{}) (lockKey string, identifier string, err error) {
			parts := strings.Split(obj.(string), "/")
			return parts[0], obj.(string), nil
		},
		WorkQueue: queue,
		locker:    locker,
	}

	s.KubeInformerFactory.Start(stopCh)

	// Test that threadiness limit is used and not exceeded
	doneCh := make(chan struct{})
	maxThreadsCh := make(chan int32, 1)
	maxThreadsCh <- 0
	defer close(maxThreadsCh)
	testItems := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		testItems = append(testItems, fmt.Sprintf("ns/%v", i))
	}
	var active, iteration int32
	threadiness := 5
	queue.On("ShuttingDown").Return(false)

	go sut.Run(threadiness, stopCh)

	for i := 0; i < len(testItems); i++ {
		i := i
		item := testItems[i]
		parts := strings.Split(item, "/")

		locker.On("TryGetLock", parts[0]).Return(true).Times(1)
		locker.On("ReleaseLock", parts[0]).Times(1)
		queue.On("Done", item).Times(1).Run(func(args mock.Arguments) {
			n := atomic.AddInt32(&iteration, 1)
			// Close doneCh when Done is called for the last item
			if int(n) == len(testItems) {
				close(doneCh)
			}
		})
		queue.On("Forget", item).Times(1)
		s.Handler.EXPECT().Sync(parts[0], parts[1], gomock.Any()).Times(1).DoAndReturn(func(namespace, name string, eventRecorder record.EventRecorder) error {
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
		// Check than max number of goroutines didn't exceed threadiness
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
