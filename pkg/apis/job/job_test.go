package job

import (
	"context"
	"os"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"strconv"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	kubeUtils "github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const anyContainerRegistry = "any.container.registry"
const egressIps = "0.0.0.0"

type RadixJobTestSuiteBase struct {
	suite.Suite
	testUtils   *test.Utils
	kubeClient  kube.Interface
	kubeUtils   *kubeUtils.Kube
	radixClient radixclient.Interface
}

func (s *RadixJobTestSuiteBase) SetupTest() {
	s.setupTest()
}

func (s *RadixJobTestSuiteBase) TearDownTest() {
	s.teardownTest()
}

func (s *RadixJobTestSuiteBase) setupTest() {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kubeUtil, _ := kubeUtils.New(kubeclient, radixclient, secretproviderfake.NewSimpleClientset())

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, anyContainerRegistry, egressIps)
	s.testUtils, s.kubeClient, s.kubeUtils, s.radixClient = &handlerTestUtils, kubeclient, kubeUtil, radixclient
}

func (s *RadixJobTestSuiteBase) teardownTest() {
	// Cleanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.JobsHistoryLimitEnvironmentVariable)
	os.Unsetenv(defaults.OperatorTenantIdEnvironmentVariable)
}

func (s *RadixJobTestSuiteBase) applyJobWithSync(jobBuilder utils.JobBuilder) (*v1.RadixJob, error) {
	rj, err := s.testUtils.ApplyJob(jobBuilder)
	if err != nil {
		return nil, err
	}

	err = s.runSync(rj)
	if err != nil {
		return nil, err
	}

	updatedJob, err := s.radixClient.RadixV1().RadixJobs(rj.GetNamespace()).Get(context.TODO(), rj.Name, metav1.GetOptions{})
	return updatedJob, err
}

func (s *RadixJobTestSuiteBase) runSync(rj *v1.RadixJob) error {
	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rj)
	err := job.OnSync()
	if err != nil {
		return err
	}

	return nil
}

func TestRadixJobTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobTestSuite))
}

type RadixJobTestSuite struct {
	RadixJobTestSuiteBase
}

func (s *RadixJobTestSuite) TestObjectSynced_StatusMissing_StatusFromAnnotation() {
	appName, cmName := "anyapp", "anycm"
	s.kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(
		context.Background(),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cmName}},
		metav1.CreateOptions{},
	)
	completedJobStatus := utils.ACompletedJobStatus().WithStep(
		utils.
			AScanAppStep().
			WithStarted(time.Now()).
			WithEnded(time.Now()).
			WithOutput(
				&v1.RadixJobStepOutput{Scan: &v1.RadixJobStepScanOutput{VulnerabilityListConfigMap: cmName}},
			))

	// Test
	job, err := s.applyJobWithSync(utils.NewJobBuilder().
		WithStatusOnAnnotation(completedJobStatus).
		WithAppName(appName).
		WithJobName("anyjob"))

	assert.NoError(s.T(), err)

	expectedStatus := completedJobStatus.Build()
	actualStatus := job.Status
	assertStatusEqual(s.T(), expectedStatus, actualStatus)
	cm, _ := s.kubeUtils.GetConfigMap(utils.GetAppNamespace(appName), cmName)
	s.ElementsMatch(GetOwnerReference(job), cm.GetOwnerReferences())
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobs_SecondJobQueued() {
	// Setup
	firstJob, _ := s.applyJobWithSync(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))

	// Test
	secondJob, _ := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("master"))

	assert.True(s.T(), secondJob.Status.Condition == v1.JobQueued)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.TODO(), firstJob, metav1.UpdateOptions{})
	s.runSync(firstJob)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.TODO(), secondJob.Name, metav1.GetOptions{})
	assert.True(s.T(), secondJob.Status.Condition == v1.JobRunning)
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobsDifferentBranch_SecondJobRunning() {
	// Setup
	s.applyJobWithSync(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))

	// Test
	secondJob, _ := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("release"))

	assert.True(s.T(), secondJob.Status.Condition != v1.JobQueued)
}

func (s *RadixJobTestSuite) TestHistoryLimit_IsBroken_FixedAmountOfJobs() {
	anyLimit := 3

	// Current cluster is active cluster
	os.Setenv(defaults.JobsHistoryLimitEnvironmentVariable, strconv.Itoa(anyLimit))

	firstJob, _ := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("FirstJob"))

	s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("SecondJob"))

	s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("ThirdJob"))

	s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("FourthJob"))

	jobs, _ := s.radixClient.RadixV1().RadixJobs(firstJob.Namespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(s.T(), anyLimit, len(jobs.Items), "Number of jobs should match limit")

	assert.False(s.T(), s.radixJobByNameExists("FirstJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("SecondJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("ThirdJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("FourthJob", jobs))

	s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("FifthJob"))

	jobs, _ = s.radixClient.RadixV1().RadixJobs(firstJob.Namespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(s.T(), anyLimit, len(jobs.Items), "Number of jobs should match limit")

	assert.False(s.T(), s.radixJobByNameExists("FirstJob", jobs))
	assert.False(s.T(), s.radixJobByNameExists("SecondJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("ThirdJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("FourthJob", jobs))
	assert.True(s.T(), s.radixJobByNameExists("FifthJob", jobs))
}

func (s *RadixJobTestSuite) TestTargetEnvironmentIsSet() {
	job, err := s.applyJobWithSync(utils.ARadixBuildDeployJob().WithJobName("test").WithBranch("master"))

	expectedEnvs := []string{"test"}

	assert.NoError(s.T(), err)

	// Master maps to Test env
	assert.Equal(s.T(), job.Spec.Build.Branch, "master")
	assert.Equal(s.T(), expectedEnvs, job.Status.TargetEnvs)
}

func (s *RadixJobTestSuite) radixJobByNameExists(name string, jobs *v1.RadixJobList) bool {
	return s.getRadixJobByName(name, jobs) != nil
}

func (s *RadixJobTestSuite) getRadixJobByName(name string, jobs *v1.RadixJobList) *v1.RadixJob {
	for _, job := range jobs.Items {
		if job.Name == name {
			return &job
		}
	}

	return nil
}

func assertStatusEqual(t *testing.T, expectedStatus, actualStatus v1.RadixJobStatus) {
	getTimestamp := func(t time.Time) string {
		return t.Format(time.RFC3339)
	}

	assert.Equal(t, getTimestamp(expectedStatus.Started.Time), getTimestamp(actualStatus.Started.Time))
	assert.Equal(t, getTimestamp(expectedStatus.Ended.Time), getTimestamp(actualStatus.Ended.Time))
	assert.Equal(t, expectedStatus.Condition, actualStatus.Condition)
	assert.Equal(t, expectedStatus.TargetEnvs, actualStatus.TargetEnvs)

	for index, expectedStep := range expectedStatus.Steps {
		assert.Equal(t, expectedStep.Name, actualStatus.Steps[index].Name)
		assert.Equal(t, expectedStep.Condition, actualStatus.Steps[index].Condition)
		assert.Equal(t, getTimestamp(expectedStep.Started.Time), getTimestamp(actualStatus.Steps[index].Started.Time))
		assert.Equal(t, getTimestamp(expectedStep.Ended.Time), getTimestamp(actualStatus.Steps[index].Ended.Time))
		assert.Equal(t, expectedStep.Components, actualStatus.Steps[index].Components)
		assert.Equal(t, expectedStep.PodName, actualStatus.Steps[index].PodName)
		assert.Equal(t, expectedStep.Output, actualStatus.Steps[index].Output)
	}
}
