package application

import (
	"context"
	"fmt"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

type controllerTestSuite struct {
	suite.Suite
	kubeClient                *fake.Clientset
	radixClient               *fakeradix.Clientset
	promClient                *prometheusfake.Clientset
	kubeUtil                  *kube.Kube
	eventRecorder             *record.FakeRecorder
	radixInformerFactory      informers.SharedInformerFactory
	kubeInformerFactory       kubeinformers.SharedInformerFactory
	mockCtrl                  *gomock.Controller
	handler                   *common.MockHandler
	synced                    chan bool
	stop                      chan struct{}
	testControllerSyncTimeout time.Duration
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.eventRecorder = &record.FakeRecorder{}
	s.radixInformerFactory = informers.NewSharedInformerFactory(s.radixClient, 0)
	s.kubeInformerFactory = kubeinformers.NewSharedInformerFactory(s.kubeClient, 0)
	s.mockCtrl = gomock.NewController(s.T())
	s.handler = common.NewMockHandler(s.mockCtrl)
	s.synced = make(chan bool)
	s.stop = make(chan struct{})
	s.testControllerSyncTimeout = 5 * time.Second
}

func (s *controllerTestSuite) TearDownTest() {
	close(s.synced)
	close(s.stop)
	s.mockCtrl.Finish()
}

func syncedChannelCallback(synced chan<- bool) func(namespace, name string, eventRecorder record.EventRecorder) error {
	return func(namespace, name string, eventRecorder record.EventRecorder) error {
		synced <- true
		return nil
	}
}

var synced chan bool

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient)

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, kubeUtil, radixClient
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler() {
	appName := "any-app"
	namespace := utils.GetAppNamespace(appName)
	appNamespace := test.CreateAppNamespace(s.kubeClient, appName)

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, s.handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(s.stop)
	s.kubeInformerFactory.Start(s.stop)

	go sut.Run(1, s.stop)

	ra := utils.ARadixApplication().WithAppName(appName).WithEnvironment("dev", "master").BuildRA()
	s.radixClient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), ra, metav1.CreateOptions{})

	s.handler.EXPECT().Sync(namespace, appName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	s.WaitForSynced("added app")
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler_On_Admin_Or_MachineUser_Change() {
	appName := "any-app"
	namespace := utils.GetAppNamespace(appName)
	appNamespace := test.CreateAppNamespace(s.kubeClient, appName)
	rr := &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: v1.RadixRegistrationSpec{MachineUser: false, AdGroups: []string{"first-group"}}}
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, s.handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(s.stop)
	s.kubeInformerFactory.Start(s.stop)

	go sut.Run(1, s.stop)

	ra := utils.ARadixApplication().WithAppName(appName).WithEnvironment("dev", "master").BuildRA()
	s.radixClient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), ra, metav1.CreateOptions{})
	s.handler.EXPECT().Sync(namespace, appName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	s.WaitForSynced("added app")

	rr.Spec.MachineUser = true
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, appName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	s.WaitForSynced("set machine-user")

	rr.Spec.MachineUser = false
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, appName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	s.WaitForSynced("unset machine-user")

	rr.Spec.AdGroups = []string{"another-group"}
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, appName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	s.WaitForSynced("unset machine-user")
}

func (s *controllerTestSuite) WaitForSynced(expectedOperation string) {
	timeout := time.NewTimer(s.testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow(fmt.Sprintf("Timeout waiting for %s", expectedOperation))
	}
}
func (s *controllerTestSuite) WaitForNotSynced(failMessage string) {
	timeout := time.NewTimer(1 * time.Second)
	select {
	case <-s.synced:
		s.FailNow(failMessage)
	case <-timeout.C:
	}
}

func startApplicationController(
	client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	handler Handler, stop chan struct{}) {
	eventRecorder := &record.FakeRecorder{}

	waitForChildrenToSync := false
	controller := NewController(client, kubeutil, radixClient, &handler,
		kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
