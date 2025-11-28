package job

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-operator/operator/common"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type jobTestSuite struct {
	common.ControllerTestSuite
	cleanup chan bool
}

func TestJobTestSuite(t *testing.T) {
	suite.Run(t, new(jobTestSuite))
}

func (s *jobTestSuite) SetupTest() {
	s.ControllerTestSuite.SetupTest()
	s.cleanup = make(chan bool)
}

func (s *jobTestSuite) Test_Controller_Calls_Handler() {
	jobName, namespace, appName := "any-job", "any-ns", "any-app-name"

	handler := NewMockHandler(s.MockCtrl)

	sut := NewController(s.Ctx, s.KubeClient, s.RadixClient, handler, s.KubeInformerFactory, s.RadixInformerFactory)
	s.RadixInformerFactory.Start(s.Ctx.Done())
	s.KubeInformerFactory.Start(s.Ctx.Done())
	go func() {
		err := sut.Run(s.Ctx, 4)
		s.Require().NoError(err)
	}()

	rj := &v1.RadixJob{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: jobName}, Spec: v1.RadixJobSpec{AppName: appName}}

	// Create RJ should sync and call cleanup
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(1).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rj, err := s.RadixClient.RadixV1().RadixJobs(rj.Namespace).Create(s.Ctx, rj, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on add RadixJob")
	s.waitForCleanup("Cleanup called on add RadixJob")

	// Update RJ spec (faked by incrementing generation) should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rj.Generation++
	rj, err = s.RadixClient.RadixV1().RadixJobs(rj.Namespace).Update(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on update RadixJob")
	s.waitForNotCleanup("Cleanup called on add RadixJob")

	// Update RJ labels should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rj.Labels = map[string]string{"key": "val"}
	rj, err = s.RadixClient.RadixV1().RadixJobs(rj.Namespace).Update(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on update RadixJob")
	s.waitForNotCleanup("Cleanup called on add RadixJob")

	// Update RJ annotations should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rj.Annotations = map[string]string{"key": "val"}
	rj, err = s.RadixClient.RadixV1().RadixJobs(rj.Namespace).Update(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on update RadixJob")
	s.waitForNotCleanup("Cleanup called on add RadixJob")

	// Update RJ status condition should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rj.Status.Condition = v1.JobRunning
	rj, err = s.RadixClient.RadixV1().RadixJobs(rj.Namespace).Update(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on update RadixJob")
	s.waitForNotCleanup("Cleanup called on add RadixJob")

	// Update RJ other status props should not sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(0).DoAndReturn(s.SyncedChannelCallback())
	rj.Status.Reconciled = metav1.Now()
	rj, err = s.RadixClient.RadixV1().RadixJobs(rj.Namespace).UpdateStatus(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync called on update RadixJob")
	s.waitForNotCleanup("Cleanup called on add RadixJob")

	childJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{OwnerReferences: jobs.GetOwnerReference(rj)}}

	// Create k8s job should not sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(0).DoAndReturn(s.SyncedChannelCallback())
	childJob, err = s.KubeClient.BatchV1().Jobs(rj.Namespace).Create(context.Background(), childJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync called on create k8s job")
	s.waitForNotCleanup("Cleanup called on create k8s job")

	// Update k8s job should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	childJob, err = s.KubeClient.BatchV1().Jobs(rj.Namespace).Update(context.Background(), childJob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on create k8s job")
	s.waitForNotCleanup("Cleanup called on create k8s job")

	// Delete k8s job should sync
	handler.EXPECT().CleanupJobHistory(gomock.Any(), appName).Times(0).Do(s.cleanupChannelCallback())
	handler.EXPECT().Sync(gomock.Any(), rj.Namespace, rj.Name).Times(1).DoAndReturn(s.SyncedChannelCallback())
	err = s.KubeClient.BatchV1().Jobs(rj.Namespace).Delete(context.Background(), childJob.Name, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync called on create k8s job")
	s.waitForNotCleanup("Cleanup called on create k8s job")
}

func (s *jobTestSuite) cleanupChannelCallback() func(ctx context.Context, appName string) error {
	return func(ctx context.Context, appName string) error {
		s.cleanup <- true
		return nil
	}
}

func (s *jobTestSuite) waitForCleanup(expectedOperation string) {
	timeout := time.NewTimer(s.TestControllerSyncTimeout)
	select {
	case <-s.cleanup:
	case <-timeout.C:
		s.FailNow(fmt.Sprintf("Timeout waiting for %s", expectedOperation))
	}
}

func (s *jobTestSuite) waitForNotCleanup(failMessage string) {
	timeout := time.NewTimer(10 * time.Millisecond)
	select {
	case <-s.cleanup:
		s.FailNow(failMessage)
	case <-timeout.C:
	}
}
