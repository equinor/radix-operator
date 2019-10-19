package job

import (
	"os"
	"strconv"
	"testing"

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

	startedJobStatus := utils.AStartedJobStatus()

	// Test
	job, err := applyJobWithSync(tu, client, kubeutils, radixclient,
		utils.ARadixBuildDeployJob().WithStatusOnAnnotation(startedJobStatus))

	assert.NoError(t, err)
	assert.Equal(t, startedJobStatus.Build(), job.Status)
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
