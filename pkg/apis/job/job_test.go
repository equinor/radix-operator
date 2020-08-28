package job

import (
	"os"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const anyContainerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface, *kubeUtils.Kube, radixclient.Interface) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kubeUtil, _ := kubeUtils.New(kubeclient, radixclient)

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, anyContainerRegistry)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient
}

func teardownTest() {
	// Celanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.JobsHistoryLimitEnvironmentVariable)
}

func TestObjectSynced_StatusMissing_StatusFromAnnotation(t *testing.T) {
	tu, client, kubeutils, radixclient := setupTest()

	completedJobStatus := utils.ACompletedJobStatus()

	// Test
	job, err := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithStatusOnAnnotation(completedJobStatus))

	assert.NoError(t, err)

	expectedStatus := completedJobStatus.Build()
	actualStatus := job.Status
	assertStatusEqual(t, expectedStatus, actualStatus)
}

func TestObjectSynced_MultipleJobs_SecondJobQueued(t *testing.T) {
	tu, client, kubeutils, radixclient := setupTest()

	// Setup
	firstJob, _ := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))

	// Test
	secondJob, _ := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("master"))

	assert.True(t, secondJob.Status.Condition == v1.JobQueued)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	radixclient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(firstJob)
	runSync(client, kubeutils, radixclient, firstJob)

	secondJob, _ = radixclient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(secondJob.Name, metav1.GetOptions{})
	assert.True(t, secondJob.Status.Condition == v1.JobRunning)
}

func TestObjectSynced_MultipleJobsDifferentBranch_SecondJobRunning(t *testing.T) {
	tu, client, kubeutils, radixclient := setupTest()

	// Setup
	applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithBranch("master"))

	// Test
	secondJob, _ := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("release"))

	assert.True(t, secondJob.Status.Condition != v1.JobQueued)
}

func TestHistoryLimit_IsBroken_FixedAmountOfJobs(t *testing.T) {
	anyLimit := 3

	tu, client, kubeutils, radixclient := setupTest()

	// Current cluster is active cluster
	os.Setenv(defaults.JobsHistoryLimitEnvironmentVariable, strconv.Itoa(anyLimit))

	firstJob, _ := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("FirstJob"))

	applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("SecondJob"))

	applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("ThirdJob"))

	applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("FourthJob"))

	jobs, _ := radixclient.RadixV1().RadixJobs(firstJob.Namespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(jobs.Items), "Number of jobs should match limit")

	assert.False(t, radixJobByNameExists("FirstJob", jobs))
	assert.True(t, radixJobByNameExists("SecondJob", jobs))
	assert.True(t, radixJobByNameExists("ThirdJob", jobs))
	assert.True(t, radixJobByNameExists("FourthJob", jobs))

	applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("FifthJob"))

	jobs, _ = radixclient.RadixV1().RadixJobs(firstJob.Namespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(jobs.Items), "Number of jobs should match limit")

	assert.False(t, radixJobByNameExists("FirstJob", jobs))
	assert.False(t, radixJobByNameExists("SecondJob", jobs))
	assert.True(t, radixJobByNameExists("ThirdJob", jobs))
	assert.True(t, radixJobByNameExists("FourthJob", jobs))
	assert.True(t, radixJobByNameExists("FifthJob", jobs))

	teardownTest()
}

func TestTargetEnvironmentIsSet(t *testing.T) {
	tu, client, kubeutils, radixclient := setupTest()

	job, err := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithJobName("test").WithBranch("master"))

	expectedEnvs := []string{"test"}

	assert.NoError(t, err)

	// Master maps to Test env
	assert.Equal(t, job.Spec.Build.Branch, "master")
	assert.Equal(t, expectedEnvs, job.Status.TargetEnvs)
}

func applyJobWithSync(tu *test.Utils, client kube.Interface, kubeutils *kubeUtils.Kube,
	radixclient radixclient.Interface, jobBuilder utils.JobBuilder) (*v1.RadixJob, error) {
	rj, err := tu.ApplyJob(jobBuilder)
	if err != nil {
		return nil, err
	}

	err = runSync(client, kubeutils, radixclient, rj)
	if err != nil {
		return nil, err
	}

	return rj, nil
}

func runSync(client kube.Interface, kubeutils *kubeUtils.Kube, radixclient radixclient.Interface, rj *v1.RadixJob) error {
	job := NewJob(client, kubeutils, radixclient, rj)
	err := job.OnSync()
	if err != nil {
		return err
	}

	return nil
}

func radixJobByNameExists(name string, jobs *v1.RadixJobList) bool {
	return getRadixJobByName(name, jobs) != nil
}

func getRadixJobByName(name string, jobs *v1.RadixJobList) *v1.RadixJob {
	for _, job := range jobs.Items {
		if job.Name == name {
			return &job
		}
	}

	return nil
}

// Problems in comparing timestamps
func assertStatusEqual(t *testing.T, expectedStatus, actualStatus v1.RadixJobStatus) {
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
	}
}

func getTimestamp(t time.Time) string {
	return t.Format("20060102150405") // YYYYMMDDHHMISS in Go
}
