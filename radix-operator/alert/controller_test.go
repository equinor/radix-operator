package alert

import (
	"context"
	"github.com/equinor/radix-operator/radix-operator/common"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	testControllerSyncTimeout = 5 * time.Second
)

type controllerTestSuite struct {
	suite.Suite
	kubeClient           *fake.Clientset
	radixClient          *fakeradix.Clientset
	promClient           *prometheusfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	radixInformerFactory informers.SharedInformerFactory
	kubeInformerFactory  kubeinformers.SharedInformerFactory
	mockCtrl             *gomock.Controller
	handler              *common.MockHandler
	synced               chan bool
	stop                 chan struct{}
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

func (s *controllerTestSuite) Test_RadixAlertEvents() {
	alertName, namespace := "any-alert", "any-ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, s.handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(s.stop)
	s.kubeInformerFactory.Start(s.stop)
	go sut.Run(1, s.stop)

	// Adding a RadixAlert should trigger sync
	s.handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})
	timeout := time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for first call")
	}

	// Updating the RadixAlert with changes should trigger a sync
	s.handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	alert.Labels = map[string]string{"foo": "bar"}
	s.radixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for second call")
	}

	// Updating the RadixAlert with no changes should not trigger a sync
	s.handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(0)
	s.radixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	timeout = time.NewTimer(1 * time.Second)
	select {
	case <-s.synced:
		s.FailNow("Sync should not be called when updating RadixAlert with no changes")
	case <-timeout.C:
	}
}

func (s *controllerTestSuite) Test_RadixRegistrationEvents() {
	alert1Name, alert2Name, namespace, appName := "alert1", "alert2", "any-ns", "any-app"
	alert1 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert1Name, Labels: map[string]string{kube.RadixAppLabel: appName}}}
	alert2 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert2Name}}
	rr := &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: v1.RadixRegistrationSpec{Owner: "first-owner", MachineUser: true, AdGroups: []string{"first-group"}}}
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, s.handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(s.stop)
	s.kubeInformerFactory.Start(s.stop)
	go sut.Run(1, s.stop)

	hasSynced := cache.WaitForCacheSync(s.stop, s.radixInformerFactory.Radix().V1().RadixRegistrations().Informer().HasSynced)
	s.True(hasSynced)

	// Initial Sync for the two alerts
	s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert1, metav1.CreateOptions{})
	s.handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	timeout := time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for initial sync of alert1")
	}
	s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert2, metav1.CreateOptions{})
	s.handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for initial sync of alert2")
	}

	// Update machineUser should trigger sync of alert1
	rr.Spec.MachineUser = false
	rr.ResourceVersion = "1"
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for sync on machineUser update")
	}

	// Update adGroups should trigger sync of alert1
	rr.Spec.AdGroups = []string{"another-group"}
	rr.ResourceVersion = "2"
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-s.synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for sync on adGroups update")
	}

	// Update other props on RR should not trigger sync of alert1
	rr.Spec.Owner = "owner"
	rr.ResourceVersion = "3"
	s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(s.synced)).Times(0)
	timeout = time.NewTimer(1 * time.Second)
	select {
	case <-s.synced:
		s.FailNow("Sync should not be called when updating other RR props")
	case <-timeout.C:
	}
}
