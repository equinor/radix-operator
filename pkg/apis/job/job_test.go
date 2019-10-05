package job

import (
	"os"
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
	kubeUtil, _ := kubeUtils.New(kubeclient)

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
