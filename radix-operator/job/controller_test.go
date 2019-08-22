package job

import (
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
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

func setupTest() (*test.Utils, kubernetes.Interface, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, radixClient
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
	tu, client, radixClient := setupTest()
	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	jobHandler := NewHandler(
		client,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startJobController(client, radixClient, jobHandler, stop)

	// Test

	// Create job should sync
	rj, _ := tu.ApplyJob(
		utils.ARadixBuildDeployJob().
			WithAppName(anyAppName))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Update  radix job should sync
	radixClient.RadixV1().RadixJobs(rj.ObjectMeta.Namespace).Update(rj)

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	// Child job should sync
	childJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: jobs.GetOwnerReference(rj),
		},
	}

	client.BatchV1().Jobs(rj.ObjectMeta.Namespace).Create(&childJob)
	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	teardownTest()
}

func startJobController(client kubernetes.Interface, radixClient radixclient.Interface, handler Handler, stop chan struct{}) {

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	eventRecorder := &record.FakeRecorder{}

	controller := NewController(
		client, radixClient, &handler,
		radixInformerFactory.Radix().V1().RadixJobs(),
		kubeInformerFactory.Batch().V1().Jobs(),
		kubeInformerFactory.Core().V1().Pods(),
		eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
