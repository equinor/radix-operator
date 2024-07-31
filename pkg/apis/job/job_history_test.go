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

type RadixJobHistoryTestSuiteBase struct {
	suite.Suite
	kubeUtils   *kube.Kube
	radixClient radixclient.Interface
}

func (s *RadixJobHistoryTestSuiteBase) SetupTest() {
	s.setupTest()
}

func (s *RadixJobHistoryTestSuiteBase) setupTest() {
	// Setup
	kubeClient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	s.kubeUtils, s.radixClient = kubeUtil, radixClient
}

func TestRadixJobTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobHistoryTestSuite))
}

type RadixJobHistoryTestSuite struct {
	RadixJobHistoryTestSuiteBase
}

func (s *RadixJobHistoryTestSuite) TestObjectSynced_StatusMissing_StatusFromAnnotation() {
	const (
		appName1 = "anyApp"
		jobName1 = "anyJob1"
		jobName2 = "anyJob2"
		jobName3 = "anyJob3"
	)
	s.createRadixJob(appName1, jobName1)

	namespace := utils.GetAppNamespace(appName1)
	radixJobList, err := s.radixClient.RadixV1().RadixJobs(namespace).List(context.Background(), metav1.ListOptions{})
	s.NoError(err)
	s.Require().Len(radixJobList.Items, 1)

	done := make(chan struct{})
	job.NewHistory(s.radixClient, s.kubeUtils, 1, job.WithDoneChannel(done)).
		Cleanup(context.Background(), appName1, jobName1)
	select {
	case <-done:
		radixJobList, err := s.radixClient.RadixV1().RadixJobs(namespace).List(context.Background(), metav1.ListOptions{})
		s.NoError(err)
		s.Require().Len(radixJobList.Items, 1)
	case <-time.After(10 * time.Second):
		s.Fail("Timed out")
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
	return &radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{
		Name: jobName,
		Labels: radixlabels.Merge(
			radixlabels.ForApplicationName(appName),
			radixlabels.ForPipelineJobName(jobName),
			radixlabels.ForPipelineJobType(),
			radixlabels.ForPipelineJobPipelineType(radixv1.BuildDeploy),
		),
	}}
}
