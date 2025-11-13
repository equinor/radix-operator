package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	testManager manager.Manager
)

var componentSpecs = []struct {
	Name         string
	Dockerfile   string
	ImageName    string
	HelmValueKey string
}{
	{
		Name:         "radix-operator",
		Dockerfile:   "operator.Dockerfile",
		ImageName:    "local-kind-repo/radix-operator",
		HelmValueKey: "image",
	},
	{
		Name:         "radix-webhook",
		Dockerfile:   "webhook.Dockerfile",
		ImageName:    "local-kind-repo/webhook",
		HelmValueKey: "radixWebhook.image",
	},
	{
		Name:         "radix-pipeline-runner",
		Dockerfile:   "pipeline.Dockerfile",
		ImageName:    "local-kind-repo/pipeline-runner",
		HelmValueKey: "radixPipelineRunner.image",
	},
	{
		Name:         "radix-job-scheduler",
		Dockerfile:   "job-scheduler.Dockerfile",
		ImageName:    "local-kind-repo/job-scheduler",
		HelmValueKey: "radixJobScheduler.image",
	},
}

// TestMain is the entry point for e2e tests
func TestMain(m *testing.M) {

	// Create a context with timeout for the entire test suite
	testContext, testCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer testCancel()

	// Generate image tag
	imageTag := internal.GenerateImageTag()
	println("Starting parallel cluster creation and image builds with tag:", imageTag)

	var testCluster *internal.KindCluster

	var eg errgroup.Group

	// Start building images
	for _, spec := range componentSpecs {
		eg.Go(func() error {
			return internal.BuildImage(testContext, spec.Dockerfile, spec.ImageName, imageTag)
		})
	}

	// Start creating Kind cluster
	eg.Go(func() error {
		cluster, err := internal.NewKindCluster(testContext, internal.KindClusterConfig{
			Name:       "radix-operator-e2e",
			KubeConfig: "",
		})
		if err != nil {
			return err
		}
		testCluster = cluster
		return nil
	})

	// Wait for both to complete
	err := eg.Wait()
	if err != nil {
		panic("failed to setup cluster or build images: " + err.Error())
	}
	println("Cluster and images ready")

	// Get kubeconfig
	kubeConfig, err := testCluster.GetKubeConfig()
	if err != nil {
		panic("failed to get kubeconfig: " + err.Error())
	}

	// Load images into Kind cluster
	println("Loading images into Kind cluster...")
	for _, spec := range componentSpecs {
		if err = testCluster.LoadImage(testContext, spec.ImageName, imageTag); err != nil {
			panic("failed to load image: " + err.Error())
		}
	}

	// Create scheme with all required types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))

	// Initialize logger for controller-runtime
	logger := initLogger()
	logrLogger := initLogr(logger)

	// Create manager
	if testManager, err = manager.New(kubeConfig, manager.Options{
		Scheme: scheme,
		Logger: logrLogger,
	}); err != nil {
		panic("failed to create manager: " + err.Error())
	}

	// Install Prometheus Operator CRDs first
	if err = internal.InstallPrometheusOperatorCRDs(testContext, testCluster.KubeConfigPath); err != nil {
		panic("failed to install Prometheus Operator CRDs: " + err.Error())
	}

	// Install Helm chart with custom image tags
	helmValues := map[string]string{
		"rbac.createApp.groups[0]": "123",
		"radixWebhook.enabled":     "true",
		"image.pullPolicy":         "IfNotPresent",
	}
	for _, spec := range componentSpecs {
		helmValues[fmt.Sprintf("%s.repository", spec.HelmValueKey)] = spec.ImageName
		helmValues[fmt.Sprintf("%s.tag", spec.HelmValueKey)] = imageTag
	}
	if err = internal.InstallRadixOperator(testContext, testCluster.KubeConfigPath, "default", "radix-operator", "../charts/radix-operator", helmValues); err != nil {
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

	// Wait for webhook deployment to be ready
	if err := WaitForDeploymentReady(testContext, testManager.GetClient(), "default", "radix-webhook"); err != nil {
		panic("failed to wait for webhook to be ready: " + err.Error())
	}

	// Run tests
	code := m.Run()

	// Cleanup cluster
	ignoreDelete := os.Getenv("RADIX_E2E_IGNORE_CLUSTER_DELETE") == "true"
	if testCluster != nil && !ignoreDelete {
		_ = testCluster.Delete(context.Background())
	}

	os.Exit(code)
}

// getClient returns the client from the manager for tests
func getClient(t *testing.T) client.Client {
	require.NotNil(t, testManager, "manager not initialized")
	return testManager.GetClient()
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

// WaitForDeploymentReady waits for a deployment to be ready using controller-runtime client
func WaitForDeploymentReady(ctx context.Context, c client.Client, namespace, name string) error {
	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		deployment := &appsv1.Deployment{}
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment)
		if err != nil {
			return false, nil // Deployment doesn't exist yet, keep waiting
		}

		// Check if deployment is ready
		if deployment.Status.ReadyReplicas > 0 && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			return true, nil
		}

		return false, nil
	})
}
