package common

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"time"
)

type ControllerTestSuite struct {
	suite.Suite
	KubeClient                *fake.Clientset
	RadixClient               *fakeradix.Clientset
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

func (s *ControllerTestSuite) SetupSuite() {
	s.KubeClient = fake.NewSimpleClientset()
	s.RadixClient = fakeradix.NewSimpleClientset()
	s.KubeUtil, _ = kube.New(s.KubeClient, s.RadixClient)
	s.PromClient = prometheusfake.NewSimpleClientset()
	s.EventRecorder = &record.FakeRecorder{}
	s.RadixInformerFactory = informers.NewSharedInformerFactory(s.RadixClient, 0)
	s.KubeInformerFactory = kubeinformers.NewSharedInformerFactory(s.KubeClient, 0)
	s.MockCtrl = gomock.NewController(s.T())
	s.Handler = NewMockHandler(s.MockCtrl)
	s.Synced = make(chan bool)
	s.Stop = make(chan struct{})
	s.TestControllerSyncTimeout = 5 * time.Second
}

func (s *ControllerTestSuite) TearDown() {
	close(s.Synced)
	close(s.Stop)
	s.MockCtrl.Finish()
}

func (s *ControllerTestSuite) WaitForSynced(expectedOperation string) {
	timeout := time.NewTimer(s.TestControllerSyncTimeout)
	select {
	case <-s.Synced:
	case <-timeout.C:
		s.FailNow(fmt.Sprintf("Timeout waiting for %s", expectedOperation))
	}
}
func (s *ControllerTestSuite) WaitForNotSynced(failMessage string) {
	timeout := time.NewTimer(1 * time.Second)
	select {
	case <-s.Synced:
		s.FailNow(failMessage)
	case <-timeout.C:
	}
}

func (s *ControllerTestSuite) SyncedChannelCallback(synced chan<- bool) func(namespace, name string, eventRecorder record.EventRecorder) error {
	return func(namespace, name string, eventRecorder record.EventRecorder) error {
		synced <- true
		return nil
	}
}
