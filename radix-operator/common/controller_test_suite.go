package common

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

// ControllerTestSuite Test suite
type ControllerTestSuite struct {
	suite.Suite
	KubeClient                *fake.Clientset
	RadixClient               *fakeradix.Clientset
	SecretProviderClient      *secretproviderfake.Clientset
	PromClient                *prometheusfake.Clientset
	KubeUtil                  *kube.Kube
	EventRecorder             *record.FakeRecorder
	RadixInformerFactory      informers.SharedInformerFactory
	KubeInformerFactory       kubeinformers.SharedInformerFactory
	MockCtrl                  *gomock.Controller
	Handler                   *MockHandler
	Synced                    chan bool
	Stop                      chan struct{}
	TestControllerSyncTimeout time.Duration
}

// SetupTest Set up the test suite
func (s *ControllerTestSuite) SetupTest() {
	s.KubeClient = fake.NewSimpleClientset()
	s.RadixClient = fakeradix.NewSimpleClientset()
	s.SecretProviderClient = secretproviderfake.NewSimpleClientset()
	s.KubeUtil, _ = kube.New(s.KubeClient, s.RadixClient, s.SecretProviderClient)
	s.PromClient = prometheusfake.NewSimpleClientset()
	s.EventRecorder = &record.FakeRecorder{}
	s.RadixInformerFactory = informers.NewSharedInformerFactory(s.RadixClient, 0)
	s.KubeInformerFactory = kubeinformers.NewSharedInformerFactory(s.KubeClient, 0)
	s.MockCtrl = gomock.NewController(s.T())
	s.Handler = NewMockHandler(s.MockCtrl)
	s.Synced = make(chan bool)
	s.Stop = make(chan struct{})
	s.TestControllerSyncTimeout = 10 * time.Second
}

// TearDownTest Tear down the test suite
func (s *ControllerTestSuite) TearDownTest() {
	close(s.Synced)
	close(s.Stop)
	s.MockCtrl.Finish()
}

// WaitForSynced Wait while Synced signal received or fail after TestControllerSyncTimeout
func (s *ControllerTestSuite) WaitForSynced(expectedOperation string) {
	timeout := time.NewTimer(s.TestControllerSyncTimeout)
	select {
	case <-s.Synced:
	case <-timeout.C:
		s.FailNow(fmt.Sprintf("Timeout waiting for %s", expectedOperation))
	}
}

// WaitForNotSynced Wait for Synced signal is not received during a second
func (s *ControllerTestSuite) WaitForNotSynced(failMessage string) {
	timeout := time.NewTimer(10 * time.Millisecond)
	select {
	case <-s.Synced:
		s.FailNow(failMessage)
	case <-timeout.C:
	}
}

// SyncedChannelCallback Callback to send a signal to the Synced
func (s *ControllerTestSuite) SyncedChannelCallback() func(ctx context.Context, namespace string, name string, eventRecorder record.EventRecorder) error {
	return func(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
		s.Synced <- true
		return nil
	}
}
