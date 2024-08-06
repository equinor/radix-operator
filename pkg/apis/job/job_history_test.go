package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
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

const (
	app1       = "any-app1"
	app2       = "any-app2"
	job1       = "any-job1"
	job2       = "any-job2"
	job3       = "any-job3"
	job4       = "any-job4"
	job5       = "any-job5"
	job6       = "any-job6"
	job7       = "any-job7"
	job8       = "any-job8"
	job9       = "any-job9"
	job10      = "any-job10"
	job11      = "any-job11"
	job12      = "any-job12"
	job13      = "any-job13"
	job14      = "any-job14"
	job15      = "any-job15"
	env1       = "dev1"
	env2       = "dev2"
	env3       = "dev3"
	envBranch1 = "dev-branch1"
	envBranch2 = "dev-branch2"
)

func (s *RadixJobHistoryTestSuite) TestJobHistory_Cleanup() {
	type appRadixJob struct {
		appName string
		jobName string
	}
	type appRadixJobsMap map[string][]string
	type scenario struct {
		name               string
		historyLimit       int
		historyPeriodLimit time.Duration
		initTest           func(radixClient radixclient.Interface)
		syncAddingRadixJob appRadixJob
		expectedRadixJobs  appRadixJobsMap
	}

	now := time.Now()
	historyPeriodLimit30Days := time.Hour * 24 * 30
	scenarios := []scenario{
		{
			name:               "No jobs deleted when count is below limit",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobRunning, true, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job1},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1},
			},
		},
		{
			name:               "No jobs deleted when count equals to limit",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job2},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2},
			},
		},
		{
			name:               "No jobs are deleted as they have RadixDeployments when count is more then limit for build-deploy",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3, job4}},
		},
		{
			name:               "One job without radix-deployment deleted when count is more then limit for build-deploy",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "One job without RadixDeploy deleted when count is more then limit for build-only",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.Build, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "One job deleted when count is more then limit for deploy-only",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.Deploy, env1, "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "One job deleted when count is more then limit for promote",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.Promote, env1, "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "One job deleted when count is more then limit for apply-config",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.ApplyConfig, "", "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "One job deleted when count is more then limit for different pipeline types",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.Build, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4}},
		},
		{
			name:               "Deleted jobs only for specific app",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app2, job1, now, radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app2, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job4},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job3, job4},
				app2: []string{job1, job2},
			},
		},
		{
			name:               "None deleted below or equal history limit",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				// s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobSucceeded, false, radixv1.BuildDeploy, env1, envBranch1) //successful job is a special case - exclude it from here
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job5, now.Add(4*time.Minute), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job6, now.Add(5*time.Minute), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job7, now.Add(6*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)          // below limit
				s.createRadixJob(radixClient, app1, job8, now.Add(7*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1) // over limit - delete this

				s.createRadixJob(radixClient, app1, job9, now.Add(9*time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job10, now.Add(11*time.Minute), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job11, now.Add(12*time.Minute), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job12, now.Add(13*time.Minute), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job13, now.Add(14*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1) // equals limit
				s.createRadixJob(radixClient, app1, job14, now.Add(15*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1) // below limit
				s.createRadixJob(radixClient, app1, job15, now.Add(16*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job15},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job2, job4, job5, job6, job7, job9, job10, job11, job12, job13, job14, job15}},
		},
		{
			name:               "Deleted only completed jobs without RadixDeployment per status",
			historyLimit:       1,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(2*time.Minute), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job5, now.Add(4*time.Minute), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job6, now.Add(5*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job7, now.Add(6*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1)

				s.createRadixJob(radixClient, app1, job8, now.Add(7*time.Minute), radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job9, now.Add(8*time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job10, now.Add(10*time.Minute), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job11, now.Add(11*time.Minute), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job12, now.Add(12*time.Minute), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job13, now.Add(13*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job14, now.Add(14*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job15, now.Add(15*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job15},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job3, job4, job6, job8, job9, job10, job11, job12, job13, job14, job15}},
		},
		{
			name:               "Deleted succeeded jobs without deployment within limit for build-deploy",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(4*time.Minute), radixv1.JobRunning, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job5, now.Add(5*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job6, now.Add(6*time.Minute), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job7, now.Add(7*time.Minute), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job8, now.Add(8*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job9, now.Add(9*time.Minute), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job10, now.Add(10*time.Minute), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job10},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job3, job4, job5, job6, job7, job8, job9, job10}},
		},
		{
			name:               "Not deleted succeeded jobs without deployment within limit for build",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job3, now.Add(4*time.Minute), radixv1.JobRunning, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job4, now.Add(6*time.Minute), radixv1.JobStopped, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job5, now.Add(7*time.Minute), radixv1.JobFailed, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job6, now.Add(8*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job7, now.Add(9*time.Minute), radixv1.JobWaiting, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job8, now.Add(10*time.Minute), radixv1.JobRunning, false, radixv1.Build, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job8},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3, job4, job5, job6, job7, job8}},
		},
		{
			name:               "Deleted succeeded jobs without deployment within limit for deploy-only",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(4*time.Minute), radixv1.JobRunning, true, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job5, now.Add(5*time.Minute), radixv1.JobRunning, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job6, now.Add(6*time.Minute), radixv1.JobStopped, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job7, now.Add(7*time.Minute), radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job8, now.Add(9*time.Minute), radixv1.JobWaiting, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job9, now.Add(10*time.Minute), radixv1.JobRunning, false, radixv1.Deploy, env1, "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job9},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job3, job4, job5, job6, job7, job8, job9}},
		},
		{
			name:               "Deleted succeeded jobs without deployment within limit for promote",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job2, now.Add(time.Minute), radixv1.JobSucceeded, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(4*time.Minute), radixv1.JobRunning, true, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job5, now.Add(5*time.Minute), radixv1.JobRunning, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job6, now.Add(6*time.Minute), radixv1.JobStopped, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job7, now.Add(7*time.Minute), radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job8, now.Add(9*time.Minute), radixv1.JobWaiting, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job9, now.Add(10*time.Minute), radixv1.JobRunning, false, radixv1.Promote, env1, "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job9},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job3, job4, job5, job6, job7, job8, job9}},
		},
		{
			name:               "Not deleted succeeded jobs without deployment within limit for apply-config",
			historyLimit:       2,
			historyPeriodLimit: historyPeriodLimit30Days,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now, radixv1.JobSucceeded, true, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job2, now.Add(3*time.Minute), radixv1.JobQueued, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job3, now.Add(5*time.Minute), radixv1.JobRunning, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job4, now.Add(6*time.Minute), radixv1.JobStopped, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job5, now.Add(7*time.Minute), radixv1.JobFailed, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job6, now.Add(8*time.Minute), radixv1.JobStoppedNoChanges, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job7, now.Add(9*time.Minute), radixv1.JobWaiting, false, radixv1.ApplyConfig, "", "")
				s.createRadixJob(radixClient, app1, job8, now.Add(10*time.Minute), radixv1.JobRunning, false, radixv1.ApplyConfig, "", "")
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job8},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3, job4, job5, job6, job7, job8}},
		},
		{
			name:               "Delete all jobs without RadixDeployment older them historyPeriodLimit",
			historyLimit:       100,
			historyPeriodLimit: time.Hour,
			initTest: func(radixClient radixclient.Interface) {
				s.createRadixJob(radixClient, app1, job1, now.Add(-100*time.Hour), radixv1.JobSucceeded, true, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job2, now.Add(-99*time.Hour), radixv1.JobSucceeded, true, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, job3, now.Add(-98*time.Hour), radixv1.JobSucceeded, true, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, job4, now.Add(-time.Minute), radixv1.JobRunning, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-87", now.Add(-100*time.Hour), radixv1.JobSucceeded, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-88", now.Add(-99*time.Hour), radixv1.JobSucceeded, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-89", now.Add(-98*time.Hour), radixv1.JobSucceeded, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-90", now.Add(-97*time.Hour), radixv1.JobSucceeded, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-91", now.Add(-96*time.Hour), radixv1.JobFailed, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-92", now.Add(-95*time.Hour), radixv1.JobFailed, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-93", now.Add(-94*time.Hour), radixv1.JobFailed, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-94", now.Add(-93*time.Hour), radixv1.JobFailed, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-95", now.Add(-96*time.Hour), radixv1.JobQueued, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-96", now.Add(-95*time.Hour), radixv1.JobQueued, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-97", now.Add(-94*time.Hour), radixv1.JobQueued, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-98", now.Add(-93*time.Hour), radixv1.JobQueued, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-99", now.Add(-96*time.Hour), radixv1.JobWaiting, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-100", now.Add(-95*time.Hour), radixv1.JobWaiting, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-101", now.Add(-94*time.Hour), radixv1.JobWaiting, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-102", now.Add(-93*time.Hour), radixv1.JobWaiting, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-103", now.Add(-96*time.Hour), radixv1.JobRunning, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-104", now.Add(-95*time.Hour), radixv1.JobRunning, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-105", now.Add(-94*time.Hour), radixv1.JobRunning, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-106", now.Add(-93*time.Hour), radixv1.JobRunning, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-107", now.Add(-96*time.Hour), radixv1.JobStopped, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-108", now.Add(-95*time.Hour), radixv1.JobStopped, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-109", now.Add(-94*time.Hour), radixv1.JobStopped, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-110", now.Add(-93*time.Hour), radixv1.JobStopped, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-111", now.Add(-96*time.Hour), radixv1.JobStoppedNoChanges, false, radixv1.BuildDeploy, env1, envBranch1)
				s.createRadixJob(radixClient, app1, "some-job-112", now.Add(-95*time.Hour), radixv1.JobStoppedNoChanges, false, radixv1.Deploy, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-113", now.Add(-94*time.Hour), radixv1.JobStoppedNoChanges, false, radixv1.Promote, env1, "")
				s.createRadixJob(radixClient, app1, "some-job-114", now.Add(-93*time.Hour), radixv1.JobStoppedNoChanges, false, radixv1.Build, env1, envBranch1)
				s.createRadixJob(radixClient, app1, job5, now, radixv1.JobRunning, true, radixv1.Build, env1, envBranch1)
			},
			syncAddingRadixJob: appRadixJob{appName: app1, jobName: job5},
			expectedRadixJobs: appRadixJobsMap{
				app1: []string{job1, job2, job3, job4, job5}},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.T().Logf("Running test: %s", ts.name)
			s.setupTest()
			ts.initTest(s.radixClient)

			err := job.NewHistory(s.radixClient, s.kubeUtils, ts.historyLimit, ts.historyPeriodLimit).
				Cleanup(context.Background(), ts.syncAddingRadixJob.appName)
			s.Require().NoError(err)

			expectedJobCount := 0
			for _, jobsMap := range ts.expectedRadixJobs {
				expectedJobCount += len(jobsMap)
			}
			actualRadixJobList, err := s.radixClient.RadixV1().RadixJobs("").List(context.Background(), metav1.ListOptions{})
			s.NoError(err)
			actualRadixJobCount := len(actualRadixJobList.Items)
			s.Equal(expectedJobCount, actualRadixJobCount, "RadixJob count")
			for _, radixJob := range actualRadixJobList.Items {
				expectedAppJobs, ok := ts.expectedRadixJobs[radixJob.Spec.AppName]
				s.True(ok, "missing RadixJobs for the app %s", radixJob.Spec.AppName)
				jobNameIndex := slice.FindIndex(expectedAppJobs, func(jobName string) bool { return radixJob.Name == jobName })
				if s.True(jobNameIndex >= 0, "unexpected RadixJob %s for the app %s", radixJob.Name, radixJob.Spec.AppName) {
					ts.expectedRadixJobs[radixJob.Spec.AppName] = append(expectedAppJobs[:jobNameIndex], expectedAppJobs[jobNameIndex+1:]...)
				}
			}
			for appName, radixJobNames := range ts.expectedRadixJobs {
				for _, radixJobName := range radixJobNames {
					s.Failf("missing RadixJob", "missing RadixJob %s for the app %s", radixJobName, appName)
				}
			}
		})
	}
}

func (s *RadixJobHistoryTestSuite) createRadixJob(radixClient radixclient.Interface, appName string, jobName string, created time.Time,
	statusCondition radixv1.RadixJobCondition, hasDeployment bool, pipelineType radixv1.RadixPipelineType, targetEnv, targetBranch string) {
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
		Create(context.Background(), createRadixJob(appName, jobName, created, statusCondition, pipelineType, targetEnv, targetBranch), metav1.CreateOptions{})
	s.Require().NoError(err)
	if hasDeployment {
		_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, targetEnv)).
			Create(context.Background(), createRadixDeployment(appName, jobName), metav1.CreateOptions{})
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
			Environments: []radixv1.Environment{
				{Name: env1, Build: radixv1.EnvBuild{From: envBranch1}},
				{Name: env2, Build: radixv1.EnvBuild{From: envBranch2}},
				{Name: env3, Build: radixv1.EnvBuild{From: envBranch2}},
			},
		},
	}
}

func createRadixJob(appName, jobName string, created time.Time, statusCondition radixv1.RadixJobCondition, pipelineType radixv1.RadixPipelineType, targetEnv, targetBranch string) *radixv1.RadixJob {
	radixJob := radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: radixlabels.Merge(
				radixlabels.ForApplicationName(appName),
				radixlabels.ForPipelineJobName(jobName),
				radixlabels.ForPipelineJobType(),
				radixlabels.ForPipelineJobPipelineType(pipelineType),
			),
		},
		Spec: radixv1.RadixJobSpec{
			AppName:      appName,
			PipeLineType: pipelineType,
		},
		Status: radixv1.RadixJobStatus{
			Created:   pointers.Ptr(metav1.NewTime(created)),
			Condition: statusCondition,
		},
	}
	switch pipelineType {
	case radixv1.Build, radixv1.BuildDeploy:
		radixJob.Spec.Build.Branch = targetBranch
	case radixv1.Deploy:
		radixJob.Spec.Deploy.ToEnvironment = targetEnv
	case radixv1.Promote:
		radixJob.Spec.Promote.ToEnvironment = targetEnv
	}
	return &radixJob
}
