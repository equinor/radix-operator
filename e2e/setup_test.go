package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	kubeClient  kubernetes.Interface
	kubeConfig  *rest.Config
	testCluster *KindCluster
	testContext context.Context
	testCancel  context.CancelFunc
)

// TestMain is the entry point for e2e tests
func TestMain(m *testing.M) {
	var err error

	// Create a context with timeout for the entire test suite
	testContext, testCancel = context.WithTimeout(context.Background(), 30*time.Minute)
	defer testCancel()

	// Create Kind cluster
	testCluster, err = NewKindCluster(testContext, KindClusterConfig{
		Name:       "radix-operator-e2e",
		KubeConfig: "",
	})
	if err != nil {
		panic("failed to create kind cluster: " + err.Error())
	}

	// Ensure cleanup
	defer func() {
		if testCluster != nil {
			_ = testCluster.Delete(context.Background())
		}
	}()

	// Get kubeconfig
	kubeConfig, err = testCluster.GetKubeConfig()
	if err != nil {
		panic("failed to get kubeconfig: " + err.Error())
	}

	// Create kubernetes client
	kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic("failed to create kubernetes client: " + err.Error())
	}

	// Install Prometheus Operator CRDs first
	helmInstaller := NewHelmInstaller(testCluster.KubeConfigPath)
	err = helmInstaller.InstallPrometheusOperatorCRDs(testContext)
	if err != nil {
		panic("failed to install Prometheus Operator CRDs: " + err.Error())
	}

	// Install Helm chart
	err = helmInstaller.InstallRadixOperator(testContext, HelmInstallConfig{
		ChartPath:   "../charts/radix-operator",
		ReleaseName: "radix-operator",
		Namespace:   "default",
		Values: map[string]interface{}{
			"rbac": map[string]interface{}{
				"createApp": map[string]interface{}{
					"groups": []string{"123"},
				},
			},
		},
	})
	if err != nil {
		panic("failed to install helm chart: " + err.Error())
	}

	// Run tests
	code := m.Run()

	os.Exit(code)
}

// getKubeClient returns the kubernetes client for tests
func getKubeClient(t *testing.T) kubernetes.Interface {
	require.NotNil(t, kubeClient, "kubernetes client not initialized")
	return kubeClient
}

// getKubeConfig returns the rest config for tests
func getKubeConfig(t *testing.T) *rest.Config {
	require.NotNil(t, kubeConfig, "kubernetes config not initialized")
	return kubeConfig
}

// getTestContext returns the test context
func getTestContext(t *testing.T) context.Context {
	require.NotNil(t, testContext, "test context not initialized")
	return testContext
}
