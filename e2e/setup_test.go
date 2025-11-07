package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	testManager manager.Manager
	kubeConfig  *rest.Config
	testCluster *internal.KindCluster
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
	testCluster, err = internal.NewKindCluster(testContext, internal.KindClusterConfig{
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

	// Create scheme with all required types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))

	// Initialize logger for controller-runtime
	logger := initLogger()
	logrLogger := initLogr(logger)

	// Create manager
	testManager, err = manager.New(kubeConfig, manager.Options{
		Scheme: scheme,
		Logger: logrLogger,
	})
	if err != nil {
		panic("failed to create manager: " + err.Error())
	}

	// Install Prometheus Operator CRDs first
	helmInstaller := internal.NewHelmInstaller(testCluster.KubeConfigPath)
	err = helmInstaller.InstallPrometheusOperatorCRDs(testContext)
	if err != nil {
		panic("failed to install Prometheus Operator CRDs: " + err.Error())
	}

	// Install Helm chart
	err = helmInstaller.InstallRadixOperator(testContext, internal.HelmInstallConfig{
		ChartPath:   "../charts/radix-operator",
		ReleaseName: "radix-operator",
		Namespace:   "default",
		Values: map[string]string{
			"rbac.createApp.groups[0]": "123",
			"radixWebhook.enabled":     "true",
		},
	})
	if err != nil {
		panic("failed to install helm chart: " + err.Error())
	}

	// Start the manager in the background
	go func() {
		if err := testManager.Start(testContext); err != nil {
			panic("failed to start manager: " + err.Error())
		}
	}()

	// Wait for the manager cache to sync
	if !testManager.GetCache().WaitForCacheSync(testContext) {
		panic("failed to wait for cache sync")
	}

	// Run tests
	code := m.Run()

	os.Exit(code)
}

// getClient returns the client from the manager for tests
func getClient(t *testing.T) client.Client {
	require.NotNil(t, testManager, "manager not initialized")
	return testManager.GetClient()
}

// getTestContext returns the test context
func getTestContext(t *testing.T) context.Context {
	require.NotNil(t, testContext, "test context not initialized")
	return testContext
}

// initLogger creates a zerolog logger for tests
func initLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	logger := zerolog.New(os.Stderr).Level(zerolog.WarnLevel).With().Timestamp().Logger()
	return logger
}

// initLogr creates a logr.Logger from zerolog and configures controller-runtime logging
func initLogr(logger zerolog.Logger) logr.Logger {
	zerologr.NameFieldName = "logger"
	zerologr.NameSeparator = "/"
	zerologr.SetMaxV(2)

	var log logr.Logger = zerologr.New(&logger)
	siglog.SetLogger(log)

	return log
}
