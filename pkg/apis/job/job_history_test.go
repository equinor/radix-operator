package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type RadixJobHistoryTestSuite struct {
	suite.Suite
	kubeUtils   *kube.Kube
	radixClient radixclient.Interface
}

func (s *RadixJobHistoryTestSuite) setupTest() {
	kubeClient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	s.kubeUtils, s.radixClient = kubeUtil, radixClient
}

func TestRadixJobHistoryTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobHistoryTestSuite))
}

func (s *RadixJobHistoryTestSuite) TestJobHistory_Cleanup() {
	const (
		app1  = "any-app1"
		app2  = "any-app2"
		job1  = "any-job1"
		job2  = "any-job2"
		job3  = "any-job3"
		job4  = "any-job4"
		job5  = "any-job5"
		job6  = "any-job6"
		job7  = "any-job7"
		job8  = "any-job8"
		job9  = "any-job9"
		job10 = "any-job10"
		job11 = "any-job11"
		job12 = "any-job12"
		job13 = "any-job13"
		job14 = "any-job14"
		job15 = "any-job15"
		job16 = "any-job16"
		job17 = "any-job17"
		job18 = "any-job18"
	)
	type appRadixJob struct {
		appName string
		jobName string
	}
	type appRadixJobsMap map[string][]string
	type scenario struct {
		name               string
		historyLimit       int
		initTest           func(radixClient radixclient.Interface)
		syncAddingRadixJob appRadixJob
		expectedRadixJobs  appRadixJobsMap
	}

	now := time.Now()
	scenarios := []scenario{
		{
			name:         "No jobs deleted when count is below limit",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobRunning, true)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job1},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1},
			},
		},
		{
			name:         "No jobs deleted when count equals to limit",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobRunning, true)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job2},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2},
			},
		},
		{
			name:         "One job deleted when count is more then limit",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobRunning, false)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job3},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3}},
		},
		{
			name:         "Now jobs deleted when count is more then limit but all jobs have radix deployments",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobRunning, false)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job3},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3}},
		},
		{
			name:         "Deleted jobs only for specific app",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobRunning, true)
				s.createRadixJob(radixClient, app2, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app2, job2, now.Add(time.Minute), radixv1.JobSucceeded, false)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job3},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3},
				app2: []string{job1, job2},
			},
		},
		{
			name:         "None deleted below or equal history limit",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobWaiting, false)
				s.createRadixJob(radixClient, app1, job5, now.Add(4*time.Minute), radixv1.JobQueued, false)
				s.createRadixJob(radixClient, app1, job6, now.Add(5*time.Minute), radixv1.JobStopped, false)
				s.createRadixJob(radixClient, app1, job7, now.Add(6*time.Minute), radixv1.JobRunning, false)          // below limit
				s.createRadixJob(radixClient, app1, job8, now.Add(6*time.Minute), radixv1.JobStoppedNoChanges, false) // over limit - delete this

				s.createRadixJob(radixClient, app1, job9, now.Add(7*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job10, now.Add(8*time.Minute), radixv1.JobFailed, false)
				s.createRadixJob(radixClient, app1, job11, now.Add(9*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job12, now.Add(10*time.Minute), radixv1.JobWaiting, false)
				s.createRadixJob(radixClient, app1, job13, now.Add(11*time.Minute), radixv1.JobQueued, false)
				s.createRadixJob(radixClient, app1, job14, now.Add(12*time.Minute), radixv1.JobStopped, false)
				s.createRadixJob(radixClient, app1, job15, now.Add(13*time.Minute), radixv1.JobStoppedNoChanges, false) // equals limit
				s.createRadixJob(radixClient, app1, job16, now.Add(14*time.Minute), radixv1.JobStoppedNoChanges, false) // below limit
				s.createRadixJob(radixClient, app1, job17, now.Add(15*time.Minute), radixv1.JobRunning, false)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job17},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3, job4, job5, job6, job7, job9, job10, job11, job12, job13, job14, job15, job16, job17}},
		},
		{
			name:         "Deleted only completed jobs per status",
			historyLimit: 1,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobWaiting, false)
				s.createRadixJob(radixClient, app1, job5, now.Add(4*time.Minute), radixv1.JobQueued, false)
				s.createRadixJob(radixClient, app1, job6, now.Add(5*time.Minute), radixv1.JobStopped, false)
				s.createRadixJob(radixClient, app1, job7, now.Add(6*time.Minute), radixv1.JobRunning, false)
				s.createRadixJob(radixClient, app1, job8, now.Add(6*time.Minute), radixv1.JobStoppedNoChanges, false)

				s.createRadixJob(radixClient, app1, job9, now.Add(7*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job10, now.Add(8*time.Minute), radixv1.JobFailed, false)
				s.createRadixJob(radixClient, app1, job11, now.Add(9*time.Minute), radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job12, now.Add(10*time.Minute), radixv1.JobWaiting, false)
				s.createRadixJob(radixClient, app1, job13, now.Add(11*time.Minute), radixv1.JobQueued, false)
				s.createRadixJob(radixClient, app1, job14, now.Add(12*time.Minute), radixv1.JobStopped, false)
				s.createRadixJob(radixClient, app1, job15, now.Add(13*time.Minute), radixv1.JobRunning, false)
				s.createRadixJob(radixClient, app1, job16, now.Add(14*time.Minute), radixv1.JobStoppedNoChanges, false)
				s.createRadixJob(radixClient, app1, job17, now.Add(15*time.Minute), radixv1.JobRunning, false)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job17},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job4, job5, job7, job12, job13, job15, job17}},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.setupTest()
			ts.initTest(s.radixClient)
			done := make(chan struct{})
			job.NewHistory(s.radixClient, s.kubeUtils, ts.historyLimit, job.WithDoneChannel(done)).
				Cleanup(context.Background(), ts.syncAddingRadixJob.appName, ts.syncAddingRadixJob.jobName)

			expectedJobCount := 0
			for _, jobsMap := range ts.expectedRadixJobs {
				expectedJobCount += len(jobsMap)
			}
			select {
			case <-done:
				actualRadixJobList, err := s.radixClient.RadixV1().RadixJobs("").List(context.Background(), metav1.ListOptions{})
				s.NoError(err)
				s.Equal(expectedJobCount, len(actualRadixJobList.Items), "RadixJob count")
				for _, radixJob := range actualRadixJobList.Items {
					appJobs, ok := ts.expectedRadixJobs[radixJob.Spec.AppName]
					s.True(ok, "missing RadixJobs for the app %s", radixJob.Spec.AppName)
					s.Contains(appJobs, radixJob.Name, "missing RadixJob %s for the app %s", radixJob.Name, radixJob.Spec.AppName)
				}
			case <-time.After(10 * time.Second):
				s.Fail("Timed out")
			}
		})
	}
}

func (s *RadixJobHistoryTestSuite) createRadixJob(radixClient radixclient.Interface, appName string, jobName1 string, created time.Time, statusCondition radixv1.RadixJobCondition, hasDeployment bool) {
	namespace := utils.GetAppNamespace(appName)
	_, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(context.Background(), appName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := radixClient.RadixV1().RadixApplications(namespace).
				Create(context.Background(), createRadixApplication(appName), metav1.CreateOptions{})
			s.Require().NoError(err)
		} else {
			s.Require().NoError(err)
		}
	}
	_, err = radixClient.RadixV1().RadixJobs(namespace).
		Create(context.Background(), createRadixJob(appName, jobName1, created, statusCondition), metav1.CreateOptions{})
	s.Require().NoError(err)
	if hasDeployment {
		_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, "dev")).
			Create(context.Background(), createRadixDeployment(appName, jobName1), metav1.CreateOptions{})
		s.Require().NoError(err)
	}
}

func createRadixDeployment(appName string, jobName string) *radixv1.RadixDeployment {
	return &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.RandString(10),
			Labels: labels.Merge(
				radixlabels.ForApplicationName(appName),
				radixlabels.ForPipelineJobName(jobName)),
		},
	}
}

func createRadixApplication(appName string) *radixv1.RadixApplication {
	return &radixv1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: radixv1.RadixApplicationSpec{
			Environments: []radixv1.Environment{{
				Name: "dev",
			}},
		},
	}
}

func createRadixJob(appName, jobName string, created time.Time, statusCondition radixv1.RadixJobCondition) *radixv1.RadixJob {
	return &radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: radixlabels.Merge(
				radixlabels.ForApplicationName(appName),
				radixlabels.ForPipelineJobName(jobName),
				radixlabels.ForPipelineJobType(),
				radixlabels.ForPipelineJobPipelineType(radixv1.BuildDeploy),
			),
		},
		Spec: radixv1.RadixJobSpec{AppName: appName},
		Status: radixv1.RadixJobStatus{
			Created:   pointers.Ptr(metav1.NewTime(created)),
			Condition: statusCondition,
		},
	}
}
