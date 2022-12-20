package scheduledjob

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_RadixScheduledJobEvents() {
	scheduledJobName, namespace := "a-schedule-job", "a-ns"
	jobName := "a-job"
	podName := "a-pod"

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go sut.Run(5, s.Stop)

	scheduledJob := &v1.RadixScheduledJob{ObjectMeta: metav1.ObjectMeta{Name: scheduledJobName, Namespace: namespace}}

	// Adding a RadixScheduledJob should trigger sync
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	scheduledJob, err := s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), scheduledJob, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on add RadixScheduledJob")

	// Updating the RadixScheduledJob with changes should trigger a sync
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	scheduledJob.Spec.JobId = "1234"
	scheduledJob, err = s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Update(context.TODO(), scheduledJob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update RadixScheduledJob")

	// Updating the RadixScheduledJob with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Update(context.TODO(), scheduledJob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when updating RadixScheduledJob with no changes")

	// Add Kubernetes Job with ownerreference to RSJ should not trigger sync
	kubejob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:            jobName,
		Namespace:       namespace,
		ResourceVersion: "1",
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "radix.equinor.com/v1", Kind: "RadixScheduledJob", Name: scheduledJobName, Controller: utils.BoolPtr(true)},
		},
	}}
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	kubejob, err = s.KubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubejob, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding k8s job")

	// Sync should not trigger on job update if resource version is unchanged
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	kubejob, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), kubejob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on k8s job update with no resource version change")

	// Sync should trigger on job update if resource version is changed
	kubejob.ResourceVersion = "2"
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	_, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), kubejob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s job update with changed resource version")

	// Add Kubernetes Pod with ownerreference to Job should not trigger sync
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:            podName,
		Namespace:       namespace,
		ResourceVersion: "1",
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "batch/v1", Kind: "Job", Name: jobName, Controller: utils.BoolPtr(true)},
		},
	}}
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	pod, err = s.KubeClient.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding k8s pod")

	// Sync should not trigger on pod update if resource version is unchanged
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	pod, err = s.KubeClient.CoreV1().Pods(namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on k8s pod update with no resource version change")

	// Sync should trigger on pod update if resource version is changed
	pod.ResourceVersion = "2"
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	_, err = s.KubeClient.CoreV1().Pods(namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s pod update with changed resource version")

	// Sync should not trigger when deleting pod
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.KubeClient.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on k8s pod deletion")

	// Sync should trigger when deleting job
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	err = s.KubeClient.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s job deletion")

	// Sync should not trigger when deleting scheduled job
	s.Handler.EXPECT().Sync(namespace, scheduledJobName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixScheduledJobs(namespace).Delete(context.Background(), scheduledJobName, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on scheduled job deletion")

}
