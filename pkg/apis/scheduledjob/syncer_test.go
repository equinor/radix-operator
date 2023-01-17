package scheduledjob

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type syncerTestSuite struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) createSyncer(forJob *radixv1.RadixScheduledJob) Syncer {
	return NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, forJob)
}

func (s *syncerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
	s.T().Setenv("RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_MEMORY", "1500Mi")
	s.T().Setenv("RADIXOPERATOR_APP_ENV_LIMITS_DEFAULT_CPU", "2000m")
}

func (s *syncerTestSuite) Test_RestoreStatus() {
	created, started, ended := v1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: v1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, v1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, v1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldRestoreStatusFromAnnotationWhenStatusEmpty() {
	created, started, ended := v1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	expectedStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&expectedStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: v1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     radixv1.RadixScheduledJobStatus{},
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, v1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, v1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldNotRestoreStatusFromAnnotationWhenStatusNotEmpty() {
	created, started, ended := v1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.Local)), v1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.Local))
	annotationStatus := radixv1.RadixScheduledJobStatus{
		Phase:   radixv1.ScheduledJobPhaseSucceeded,
		Reason:  "any-reason",
		Message: "any-message",
		Created: &created,
		Started: &started,
		Ended:   &ended,
	}
	statusBytes, err := json.Marshal(&annotationStatus)
	s.Require().NoError(err)

	jobName, namespace := "any-job", "any-ns"
	expectedStatus := radixv1.RadixScheduledJobStatus{Phase: radixv1.ScheduledJobPhaseFailed}
	job := &radixv1.RadixScheduledJob{
		ObjectMeta: v1.ObjectMeta{Name: jobName, Annotations: map[string]string{kube.RestoredStatusAnnotation: string(statusBytes)}},
		Status:     expectedStatus,
	}
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), job, v1.CreateOptions{})
	s.Require().NoError(err)
	sut := s.createSyncer(job)
	s.Require().NoError(sut.OnSync())
	job, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, v1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedStatus, job.Status)
}

func (s *syncerTestSuite) Test_ShouldSkipReconcileResourcesWhenJobStatusIsDone() {
	donePhases := []radixv1.RadixScheduledJobPhase{radixv1.ScheduledJobPhaseFailed, radixv1.ScheduledJobPhaseStopped, radixv1.ScheduledJobPhaseSucceeded}

	for i, phase := range donePhases {
		s.Run(string(phase), func() {
			jobName, namespace := fmt.Sprintf("any-job-%d", i), "any-ns"
			expectedStatus := radixv1.RadixScheduledJobStatus{Phase: phase}
			scheduledjob := &radixv1.RadixScheduledJob{
				ObjectMeta: v1.ObjectMeta{Name: jobName},
				Status:     expectedStatus,
			}
			scheduledjob, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), scheduledjob, v1.CreateOptions{})
			s.Require().NoError(err)
			sut := s.createSyncer(scheduledjob)
			s.Require().NoError(sut.OnSync())
			scheduledjob, err = s.radixClient.RadixV1().RadixScheduledJobs(namespace).Get(context.Background(), jobName, v1.GetOptions{})
			s.Require().NoError(err)
			s.Equal(expectedStatus, scheduledjob.Status)
			jobs, err := s.kubeClient.BatchV1().Jobs(v1.NamespaceAll).List(context.Background(), v1.ListOptions{})
			s.Require().NoError(err)
			s.Len(jobs.Items, 0)
			services, err := s.kubeClient.CoreV1().Services(v1.NamespaceAll).List(context.Background(), v1.ListOptions{})
			s.Require().NoError(err)
			s.Len(services.Items, 0)
		})
	}

}

func (s *syncerTestSuite) Test_ResourceReconciled() {
	rsjName, jobName, jobImage, namespace, rdName, secretName, secretKey := "any-job", "compute", "any-image", "any-ns", "any-rd", "any-secret", "any-secret-key"
	// payloadPath := "/mnt/any/path"
	rsj := &radixv1.RadixScheduledJob{
		ObjectMeta: v1.ObjectMeta{Name: rsjName},
		Spec: radixv1.RadixScheduledJobSpec{
			TimeLimitSeconds: numbers.Int64Ptr(1000),
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: rdName},
				Job:                  jobName,
			},
			PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: secretName},
				Key:                  secretKey,
			},
		},
	}
	rd := &radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Name: rdName},
		Spec: radixv1.RadixDeploymentSpec{
			Jobs: []radixv1.RadixDeployJobComponent{
				{
					Name:  jobName,
					Image: jobImage,
				},
			},
		},
	}
	rsj, err := s.radixClient.RadixV1().RadixScheduledJobs(namespace).Create(context.Background(), rsj, v1.CreateOptions{})
	s.Require().NoError(err)
	_, err = s.radixClient.RadixV1().RadixDeployments(namespace).Create(context.Background(), rd, v1.CreateOptions{})
	s.Require().NoError(err)

	sut := s.createSyncer(rsj)
	err = sut.OnSync()
	s.Require().NoError(err)
}

// All resources (jobs and services) created according to spec - split job features (volumes, keyvaults etc)
// Phase Waiting
// Phase Running
// Phase Completed
// Phase Failed
// Reason and Status from latest Pod when Phase in Failed or Waiting

// Delete Job when stop is set to true
// Missing RD => status pending
// RD exist but missing job

//
