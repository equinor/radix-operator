package alert

import (
	"context"
	"fmt"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"

	"github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

type handlerTestSuite struct {
	suite.Suite
	kubeClient    *fake.Clientset
	radixClient   *fakeradix.Clientset
	promClient    *prometheusfake.Clientset
	kubeUtil      *kube.Kube
	eventRecorder *record.FakeRecorder
	mockCtrl      *gomock.Controller
	syncerFactory *alert.MockAlertSyncerFactory
	syncer        *alert.MockAlertSyncer
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}

func (s *handlerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient)
	s.kubeUtil.WithSecretsProvider(secretproviderfake.NewSimpleClientset())
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
	s.syncer.EXPECT().OnSync().Times(0)
	err := sut.Sync("any-ns", "any-alert", s.eventRecorder)
	s.Nil(err)
}

func (s *handlerTestSuite) Test_RadixAlertExist_AlertSyncerReturnError() {
	alertName, namespace := "alert", "ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})
	expectedError := fmt.Errorf("error")

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, WithAlertSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateAlertSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, alert).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(expectedError).Times(1)
	actualError := sut.Sync(namespace, alertName, s.eventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixAlertExist_AlertSyncerReturnNil() {
	alertName, namespace := "alert", "ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}
	alert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, WithAlertSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateAlertSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, alert).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(nil).Times(1)
	err := sut.Sync(namespace, alertName, s.eventRecorder)
	s.Nil(err)
}
