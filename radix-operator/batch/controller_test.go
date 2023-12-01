package batch

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_RadixBatchEvents() {
	batchName, namespace := "a-batch-job", "a-ns"
	jobName := "a-job"

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go func() {
		err := sut.Run(5, s.Stop)
		if err != nil {
			panic(err)
		}
	}()

	batch := &v1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: batchName, Namespace: namespace}}

	// Adding a RadixBatch should trigger sync
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	batch, err := s.RadixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), batch, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on add RadixBatch")

	// Updating the RadixBatch with changes should trigger a sync
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	batch.Spec.RadixDeploymentJobRef.Job = "newjob"
	batch, err = s.RadixClient.RadixV1().RadixBatches(namespace).Update(context.TODO(), batch, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update RadixBatch")

	// Updating the RadixBatch with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixBatches(namespace).Update(context.TODO(), batch, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when updating RadixBatch with no changes")

	// Add Kubernetes Job with ownerreference to RadixBatch should not trigger sync
	kubejob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:            jobName,
		Namespace:       namespace,
		ResourceVersion: "1",
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: radix.APIVersion, Kind: radix.KindRadixBatch, Name: batchName, Controller: utils.BoolPtr(true)},
		},
	}}
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	kubejob, err = s.KubeClient.BatchV1().Jobs(namespace).Create(context.Background(), kubejob, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding k8s job")

	// Sync should not trigger on job update if resource version is unchanged
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	kubejob, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), kubejob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on k8s job update with no resource version change")

	// Sync should trigger on job update if resource version is changed
	kubejob.ResourceVersion = "2"
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	_, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), kubejob, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s job update with changed resource version")

	// Sync should trigger when deleting job
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	err = s.KubeClient.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s job deletion")

	// Sync should not trigger when deleting batch
	s.Handler.EXPECT().Sync(namespace, batchName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixBatches(namespace).Delete(context.Background(), batchName, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on batch deletion")

}
