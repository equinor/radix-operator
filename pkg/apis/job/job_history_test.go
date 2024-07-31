package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (s *RadixJobHistoryTestSuite) TestObjectSynced_Deleted() {
	const (
		appName1 = "any-app1"
		appName2 = "any-app2"
		jobName1 = "any-job1"
		jobName2 = "any-job2"
		jobName3 = "any-job3"
	)
	type radixJobsMap map[string]struct{}
	type appRadixJobsMap map[string]radixJobsMap
	type scenario struct {
		name              string
		historyLimit      int
		initTest          func()
		expectedRadixJobs appRadixJobsMap
	}

	scenarios := []scenario{
		{
			name:         "No jobs deleted",
			historyLimit: 1,
			initTest: func() {
				s.createRadixJob(appName1, jobName1)
			},
			expectedRadixJobs: appRadixJobsMap{appName1: radixJobsMap{jobName1: struct{}{}}},
		},
	}

	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.setupTest()
			ts.initTest()
			s.T().Run("Cleanup", func(t *testing.T) {

				done := make(chan struct{})
				job.NewHistory(s.radixClient, s.kubeUtils, ts.historyLimit, job.WithDoneChannel(done)).
					Cleanup(context.Background(), appName1, jobName1)

				expectedJobCount := 0
				for _, jobsMap := range ts.expectedRadixJobs {
					expectedJobCount += len(jobsMap)
				}
				select {
				case <-done:
					actualRadixJobList, err := s.radixClient.RadixV1().RadixJobs(utils.GetAppNamespace(appName1)).List(context.Background(), metav1.ListOptions{})
					s.NoError(err)
					s.Len(actualRadixJobList.Items, expectedJobCount, "RadixJob count")
					for _, radixJob := range actualRadixJobList.Items {
						jobsMap, ok := ts.expectedRadixJobs[radixJob.Spec.AppName]
						s.True(ok, "missing RadixJobs for the app %s", radixJob.Spec.AppName)
						_, ok = jobsMap[radixJob.Name]
						s.True(ok, "missing RadixJob %s for the app %s", radixJob.Name, radixJob.Spec.AppName)
					}
					s.Equal(jobName1, actualRadixJobList.Items[0].Name)
				case <-time.After(10 * time.Second):
					s.Fail("Timed out")
				}
			})
		})

	}
}

func (s *RadixJobHistoryTestSuite) createRadixJob(appName string, jobName1 string) error {
	namespace := utils.GetAppNamespace(appName)
	_, err := s.radixClient.RadixV1().RadixJobs(namespace).
		Create(context.Background(), createRadixJob(appName, jobName1), metav1.CreateOptions{})
	s.Require().NoError(err)
	return err
}

func createRadixJob(appName, jobName string) *radixv1.RadixJob {
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
	}
}
