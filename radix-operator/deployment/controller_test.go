package deployment

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface, prometheusclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient)
	prometheusclient := prometheusfake.NewSimpleClientset()
	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, kubeUtil, radixClient, prometheusclient
}

func teardownTest() {
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	anyAppName := "test-app"
	anyEnvironment := "qa"

	// Setup
	tu, client, kubeUtil, radixClient, prometheusclient := setupTest()

	client.CoreV1().Namespaces().Create(
		context.TODO(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: utils.GetEnvironmentNamespace(anyAppName, anyEnvironment),
				Labels: map[string]string{
					kube.RadixAppLabel: anyAppName,
					kube.RadixEnvLabel: anyEnvironment,
				},
			},
		},
		metav1.CreateOptions{})

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)

	deploymentHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		prometheusclient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startDeploymentController(client, kubeUtil, radixClient, radixInformerFactory, kubeInformerFactory, deploymentHandler, stop)

	// Test

	// Create deployment should sync
	rd, _ := tu.ApplyDeployment(
		utils.ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment))

	op, ok := <-synced
	assert.True(t, ok)
	assert.True(t, op)

	syncedRd, _ := radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	lastReconciled := syncedRd.Status.Reconciled
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")

	// Update deployment should sync. Only actual updates will be handled by the controller
	noReplicas := 0
	rd.Spec.Components[0].Replicas = &noReplicas
	radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(context.TODO(), rd, metav1.UpdateOptions{})

	op, ok = <-synced
	assert.True(t, ok)
	assert.True(t, op)

	syncedRd, _ = radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")
	assert.NotEqual(t, lastReconciled, syncedRd.Status.Reconciled)
	lastReconciled = syncedRd.Status.Reconciled

	// Delete service should sync
	services, _ := client.CoreV1().Services(rd.ObjectMeta.Namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "radix-app=test-app",
		})

	for _, aservice := range services.Items {
		client.CoreV1().Services(rd.ObjectMeta.Namespace).Delete(context.TODO(), aservice.Name, metav1.DeleteOptions{})

		op, ok = <-synced
		assert.True(t, ok)
		assert.True(t, op)
	}

	syncedRd, _ = radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")
	assert.NotEqual(t, lastReconciled, syncedRd.Status.Reconciled)
	lastReconciled = syncedRd.Status.Reconciled

	teardownTest()
}

func startDeploymentController(client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	handler Handler, stop chan struct{}) {

	eventRecorder := &record.FakeRecorder{}

	waitForChildrenToSync := false
	controller := NewController(
		client, kubeutil, radixClient, &handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
