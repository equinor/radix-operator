package batch

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
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
	promClient           *prometheusfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	mockCtrl             *gomock.Controller
	syncerFactory        *batch.MockSyncerFactory
	syncer               *batch.MockSyncer
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}

func (s *handlerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.secretproviderclient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.secretproviderclient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.eventRecorder = &record.FakeRecorder{}
	s.mockCtrl = gomock.NewController(s.T())
	s.syncerFactory = batch.NewMockSyncerFactory(s.mockCtrl)
	s.syncer = batch.NewMockSyncer(s.mockCtrl)
}

func (s *handlerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *handlerTestSuite) Test_RadixScheduleJobNotFound() {
	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync().Times(0)
	err := sut.Sync("any-ns", "any-job", s.eventRecorder)
	s.Nil(err)
}

func (s *handlerTestSuite) Test_RadixScheduledExist_SyncerError() {
	jobName, namespace := "any-job", "ns"
	job := &v1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: jobName}}
	job, _ = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	expectedError := fmt.Errorf("error")

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(s.kubeClient, s.kubeUtil, s.radixClient, job).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(expectedError).Times(1)
	actualError := sut.Sync(namespace, jobName, s.eventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixScheduledExist_SyncerNoError() {
	jobName, namespace := "any-job", "ns"
	job := &v1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: jobName}}
	job, _ = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(s.kubeClient, s.kubeUtil, s.radixClient, job).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(nil).Times(1)
	err := sut.Sync(namespace, jobName, s.eventRecorder)
	s.Nil(err)
}
