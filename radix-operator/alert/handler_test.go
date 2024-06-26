package alert

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type handlerTestSuite struct {
	suite.Suite
	kubeClient           *fake.Clientset
	radixClient          *fakeradix.Clientset
	secretproviderclient *secretproviderfake.Clientset
	kedaClient           *kedafake.Clientset
	promClient           *prometheusfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	mockCtrl             *gomock.Controller
	syncerFactory        *alert.MockAlertSyncerFactory
	syncer               *alert.MockAlertSyncer
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}

func (s *handlerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.secretproviderclient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, s.secretproviderclient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.eventRecorder = &record.FakeRecorder{}
	s.mockCtrl = gomock.NewController(s.T())
	s.syncerFactory = alert.NewMockAlertSyncerFactory(s.mockCtrl)
	s.syncer = alert.NewMockAlertSyncer(s.mockCtrl)
}

func (s *handlerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *handlerTestSuite) Test_RadixAlertNotFound() {
	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, WithAlertSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateAlertSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync(gomock.Any()).Times(0)
	err := sut.Sync(context.Background(), "any-ns", "any-alert", s.eventRecorder)
	s.Nil(err)
}

func (s *handlerTestSuite) Test_RadixAlertExist_AlertSyncerReturnError() {
	alertName, namespace := "alert", "ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})
	expectedError := fmt.Errorf("error")

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, WithAlertSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateAlertSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, alert).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(expectedError).Times(1)
	actualError := sut.Sync(context.Background(), namespace, alertName, s.eventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixAlertExist_AlertSyncerReturnNil() {
	alertName, namespace := "alert", "ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, WithAlertSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateAlertSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, alert).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(nil).Times(1)
	err := sut.Sync(context.Background(), namespace, alertName, s.eventRecorder)
	s.Nil(err)
}
