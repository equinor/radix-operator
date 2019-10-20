package job

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
)

var synced chan bool

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient)

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, kubeUtil, radixClient
}

func teardownTest() {
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	anyAppName := "test-app"

	// Setup
	tu, client, kubeUtil, radixClient := setupTest()
	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	jobHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startJobController(client, kubeUtil, radixClient, jobHandler, stop)

	// Test

	// Create job should sync
	rj, _ := tu.ApplyJob(
		utils.ARadixBuildDeployJob().
			WithAppName(anyAppName))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Child job should sync
	childJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: jobs.GetOwnerReference(rj),
		},
	}

	// Only update of Kubernetes Job is something that the job-controller handles
	client.BatchV1().Jobs(rj.ObjectMeta.Namespace).Create(&childJob)
	childJob.ObjectMeta.ResourceVersion = "1234"
	client.BatchV1().Jobs(rj.ObjectMeta.Namespace).Update(&childJob)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Update  radix job should sync. Controller will skip if an update
	// changes nothing, except for spec or metadata, labels or annotations
	rj.Spec.Stop = true
	radixClient.RadixV1().RadixJobs(rj.ObjectMeta.Namespace).Update(rj)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	teardownTest()
}

func startJobController(client kubernetes.Interface, kubeutil *kube.Kube,
	radixClient radixclient.Interface, handler Handler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewController(
		client, kubeutil, radixClient, &handler,
		kubeInformerFactory,
		radixInformerFactory,
		eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
