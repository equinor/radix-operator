package scheduledjob

import (
	"context"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_RadixScheduledJobEvents() {
	resourceName, namespace := "any-job", "any-ns"

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go sut.Run(5, s.Stop)

	// Adding a RadixAlert should trigger sync
	s.Handler.EXPECT().Sync(namespace, resourceName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	scheduledJob, _ := s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), &v1.RadixScheduledJob{ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace}}, metav1.CreateOptions{})
	s.WaitForSynced("first call")

	// Updating the RadixAlert with changes should trigger a sync
	s.Handler.EXPECT().Sync(namespace, resourceName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	scheduledJob.Spec.JobId = "1234"
	s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Update(context.TODO(), scheduledJob, metav1.UpdateOptions{})
	s.WaitForSynced("second call")

	// Updating the RadixAlert with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(namespace, resourceName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Update(context.TODO(), scheduledJob, metav1.UpdateOptions{})
	s.WaitForNotSynced("Sync should not be called when updating RadixScheduledJob with no changes")
}
