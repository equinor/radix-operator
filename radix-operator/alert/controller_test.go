package alert

import (
	"context"
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
}

func syncedChannelCallback(synced chan<- bool) func(namespace, name string, eventRecorder record.EventRecorder) error {
	return func(namespace, name string, eventRecorder record.EventRecorder) error {
		synced <- true
		return nil
	}
}

func (s *controllerTestSuite) Test_RadixAlertEvents() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	handler := NewMockHandler(ctrl)
	synced := make(chan bool)
	defer close(synced)
	stop := make(chan struct{})
	defer close(stop)

	alertName, namespace := "any-alert", "any-ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(stop)
	s.kubeInformerFactory.Start(stop)
	go sut.Run(1, stop)

	// Adding a RadixAlert should trigger sync
	handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})
	timeout := time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for first call")
	}

	// Updating the RadixAlert with changes should trigger a sync
	handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	alert.Labels = map[string]string{"foo": "bar"}
	s.radixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for second call")
	}

	// Updating the RadixAlert with no changes should not trigger a sync
	handler.EXPECT().Sync(namespace, alertName, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(0)
	s.radixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	timeout = time.NewTimer(1 * time.Second)
	select {
	case <-synced:
		s.FailNow("Sync should not be called when updating RadixAlert with no changes")
	case <-timeout.C:
	}
}

func (s *controllerTestSuite) Test_RadixRegistrationEvents() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	handler := NewMockHandler(ctrl)
	synced := make(chan bool)
	defer close(synced)
	stop := make(chan struct{})
	defer close(stop)

	alert1Name, alert2Name, namespace, appName := "alert1", "alert2", "any-ns", "any-app"
	alert1 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert1Name, Labels: map[string]string{kube.RadixAppLabel: appName}}}
	alert2 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert2Name}}
	rr := &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: v1.RadixRegistrationSpec{Owner: "first-owner", MachineUser: true, AdGroups: []string{"first-group"}}}
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})

	sut := NewController(s.kubeClient, s.kubeUtil, s.radixClient, handler, s.kubeInformerFactory, s.radixInformerFactory, false, s.eventRecorder)
	s.radixInformerFactory.Start(stop)
	s.kubeInformerFactory.Start(stop)
	go sut.Run(1, stop)

	hasSynced := cache.WaitForCacheSync(stop, s.radixInformerFactory.Radix().V1().RadixRegistrations().Informer().HasSynced)
	s.True(hasSynced)

	// Initial Sync for the two alerts
	s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert1, metav1.CreateOptions{})
	handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	timeout := time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for initial sync of alert1")
	}
	s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert2, metav1.CreateOptions{})
	handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for initial sync of alert2")
	}

	// Update machineUser should trigger sync of alert1
	rr.Spec.MachineUser = false
	rr.ResourceVersion = "1"
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for sync on machineUser update")
	}

	// Update adGroups should trigger sync of alert1
	rr.Spec.AdGroups = []string{"another-group"}
	rr.ResourceVersion = "2"
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(1)
	timeout = time.NewTimer(testControllerSyncTimeout)
	select {
	case <-synced:
	case <-timeout.C:
		s.FailNow("Timeout waiting for sync on adGroups update")
	}

	// Update other props on RR should not trigger sync of alert1
	rr.Spec.Owner = "owner"
	rr.ResourceVersion = "3"
	s.radixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	handler.EXPECT().Sync(namespace, alert1Name, s.eventRecorder).DoAndReturn(syncedChannelCallback(synced)).Times(0)
	timeout = time.NewTimer(1 * time.Second)
	select {
	case <-synced:
		s.FailNow("Sync should not be called when updating other RR props")
	case <-timeout.C:
	}
}
