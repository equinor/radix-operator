package batch

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/batch"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/radix-operator/batch/internal"
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
	promClient           *prometheusfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	mockCtrl             *gomock.Controller
	syncerFactory        *internal.MockSyncerFactory
	syncer               *batch.MockSyncer
	kedaClient           *kedafake.Clientset
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
	s.syncerFactory = internal.NewMockSyncerFactory(s.mockCtrl)
	s.syncer = batch.NewMockSyncer(s.mockCtrl)
}

func (s *handlerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *handlerTestSuite) Test_RadixScheduleJobNotFound() {
	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, &config.Config{}, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync(gomock.Any()).Times(0)
	err := sut.Sync(context.Background(), "any-ns", "any-job", s.eventRecorder)
	s.Nil(err)
}

func (s *handlerTestSuite) Test_RadixScheduledExist_SyncerError() {
	jobName, namespace := "any-job", "ns"
	job := &v1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: jobName}}
	job, _ = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	expectedError := fmt.Errorf("error")
	cfg := &config.Config{}

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, cfg, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(s.kubeClient, s.kubeUtil, s.radixClient, job, cfg).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(expectedError).Times(1)
	actualError := sut.Sync(context.Background(), namespace, jobName, s.eventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixScheduledExist_SyncerNoError() {
	jobName, namespace := "any-job", "ns"
	job := &v1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: jobName}}
	job, _ = s.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	cfg := &config.Config{}

	sut := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, cfg, WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(s.kubeClient, s.kubeUtil, s.radixClient, job, cfg).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(nil).Times(1)
	err := sut.Sync(context.Background(), namespace, jobName, s.eventRecorder)
	s.Nil(err)
}
