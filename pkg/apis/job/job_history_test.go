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
		app1 = "any-app1"
		app2 = "any-app2"
		job1 = "any-job1"
		job2 = "any-job2"
		job3 = "any-job3"
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
			name:         "One job deleted",
			historyLimit: 2,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, true)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobRunning, true)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job3},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3}},
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
				actualRadixJobList, err := s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(app1)).List(context.Background(), metav1.ListOptions{})
				s.NoError(err)
				s.Len(actualRadixJobList.Items, expectedJobCount, "RadixJob count")
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
