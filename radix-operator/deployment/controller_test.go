package deployment

import (
	"context"
	"os"
	"testing"
	"time"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func setupTest(t *testing.T) (*test.Utils, *kubefake.Clientset, *kube.Kube, *fakeradix.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *certfake.Clientset) {
	client := kubefake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(client, radixClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)
	kedaClient := kedafake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()
	return &handlerTestUtils, client, kubeUtil, radixClient, kedaClient, prometheusClient, certClient
}

func teardownTest() {
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func Test_Controller_Calls_Handler(t *testing.T) {
	anyAppName := "test-app"
	anyEnvironment := "qa"

	ctx, stopFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopFn()

	synced := make(chan bool)
	defer close(synced)

	// Setup
	tu, client, kubeUtil, radixClient, kedaClient, prometheusclient, certClient := setupTest(t)

	_, err := client.CoreV1().Namespaces().Create(
		ctx,
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
	require.NoError(t, err)

	rd, err := tu.ApplyDeployment(
		context.Background(),
		utils.ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment))
	require.NoError(t, err)

	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)

	deploymentHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		kedaClient,
		prometheusclient,
		certClient,
		&config.Config{},
		WithHasSyncedCallback(func(syncedOk bool) { synced <- syncedOk }),
	)
	go func() {
		err := startDeploymentController(ctx, client, radixClient, radixInformerFactory, kubeInformerFactory, deploymentHandler)
		require.NoError(t, err)
	}()

	// Test

	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	syncedRd, err := radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(ctx, rd.GetName(), metav1.GetOptions{})
	require.NoError(t, err)
	lastReconciled := syncedRd.Status.Reconciled
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")

	// Update deployment should sync. Only actual updates will be handled by the controller
	noReplicas := 0
	rd.Spec.Components[0].Replicas = &noReplicas
	_, err = radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(ctx, rd, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case op, ok := <-synced:
		assert.True(t, op)
		assert.True(t, ok)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	syncedRd, _ = radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(ctx, rd.GetName(), metav1.GetOptions{})
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")
	assert.NotEqual(t, lastReconciled, syncedRd.Status.Reconciled)
	lastReconciled = syncedRd.Status.Reconciled

	// Delete service should sync
	services, _ := client.CoreV1().Services(rd.ObjectMeta.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "radix-app=test-app",
		})

	for _, aservice := range services.Items {
		err := client.CoreV1().Services(rd.ObjectMeta.Namespace).Delete(ctx, aservice.Name, metav1.DeleteOptions{})
		require.NoError(t, err)
		select {
		case op, ok := <-synced:
			assert.True(t, op)
			assert.True(t, ok)
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	}

	syncedRd, _ = radixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Get(ctx, rd.GetName(), metav1.GetOptions{})
	assert.Truef(t, !lastReconciled.Time.IsZero(), "Reconciled on status should have been set")
	assert.NotEqual(t, lastReconciled, syncedRd.Status.Reconciled)
	lastReconciled = syncedRd.Status.Reconciled

	teardownTest()
}

func startDeploymentController(ctx context.Context, client kubernetes.Interface, radixClient radixclient.Interface, radixInformerFactory informers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, handler *Handler) error {

	eventRecorder := &record.FakeRecorder{}

	const waitForChildrenToSync = false
	controller := NewController(
		ctx,
		client, radixClient, handler,
		kubeInformerFactory,
		radixInformerFactory,
		waitForChildrenToSync,
		eventRecorder)

	kubeInformerFactory.Start(ctx.Done())
	radixInformerFactory.Start(ctx.Done())
	return controller.Run(ctx, 4)
}
