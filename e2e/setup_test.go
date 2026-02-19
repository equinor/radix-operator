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
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
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

type Config struct {
	SetupParallelism     uint `envconfig:"E2E_SETUP_PARALLELISM" default:"0" desc:"Limits the number of active goroutines for building images and setting up kind cluster. Value 0 indicates no limit."`
	RemoveImagesOnFinish bool `envconfig:"E2E_REMOVE_IMAGES_ON_FINISH" default:"true" desc:"Remove test images after test finish."`
}

// TestMain is the entry point for e2e tests
func TestMain(m *testing.M) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		_ = envconfig.Usage("", &cfg)
		log.Fatal().Err(err).Msg("failed to parse process config")
	}
	fmt.Printf("Config:\n  SetupParallelism: %v\n  RemoveImagesOnFinish: %v\n", cfg.SetupParallelism, cfg.RemoveImagesOnFinish)

	// Create a context with timeout for the entire test suite
	testContext, testCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer testCancel()

	// Generate image tag
	imageTag := internal.GenerateImageTag()
	println("Starting parallel cluster creation and image builds with tag:", imageTag)
	var eg errgroup.Group
	if cfg.SetupParallelism > 0 {
		eg.SetLimit(int(cfg.SetupParallelism))
	}

	// Start creating Kind cluster
	var testCluster *internal.KindCluster
	eg.Go(func() error {
		var err error
		testCluster, err = internal.NewKindCluster(testContext)
		return err
	})

	// Start building images
	for _, spec := range componentSpecs {
		eg.Go(func() error {
			return internal.BuildImage(testContext, spec.Dockerfile, spec.ImageName, imageTag)
		})
	}

	// Wait for both to complete
	err = eg.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup cluster or build images")
	}
	println("Cluster and images ready")

	// Get kubeconfig
	kubeConfig, err := testCluster.GetKubeConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get kubeconfig")
	}

	// Load images into Kind cluster
	println("Loading images into Kind cluster...")
	for _, spec := range componentSpecs {
		if err = testCluster.LoadImage(testContext, spec.ImageName, imageTag); err != nil {
			log.Fatal().Err(err).Msg("failed to load image")
		}
	}

	// Create scheme with all required types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))
	utilruntime.Must(gatewayapiv1.Install(scheme))

	// Initialize logger for controller-runtime
	logger := initLogger()
	logrLogger := initLogr(logger)

	// Create manager
	if testManager, err = manager.New(kubeConfig, manager.Options{
		Scheme: scheme,
		Logger: logrLogger,
	}); err != nil {
		log.Fatal().Err(err).Msg("failed to create manager")
	}

	// Install Prometheus Operator CRDs first
	if err = internal.InstallPrometheusOperatorCRDs(testContext, testCluster.KubeConfigPath); err != nil {
		log.Fatal().Err(err).Msg("failed to install Prometheus Operator CRDs")
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
	if err = internal.InstallRadixOperator(testContext, testCluster.KubeConfigPath, "radix-system", "radix-operator", "../charts/radix-operator", helmValues); err != nil {
		log.Fatal().Err(err).Msg("failed to install radix-operator helm chart")
	}

	if err = internal.InstallGatewayApiCRDs(testContext, testCluster.KubeConfigPath); err != nil {
		log.Fatal().Err(err).Msg("failed to install Gateway API CRDs")
	}

	// Start the manager in the background
	go func() {
		if err := testManager.Start(testContext); err != nil {
			log.Fatal().Err(err).Msg("failed to start manager")
		}
	}()

	// Wait for the manager cache to sync
	if !testManager.GetCache().WaitForCacheSync(testContext) {
		log.Fatal().Msg("failed to wait for cache sync")
	}

	// Wait for webhook deployment to be ready
	if err := WaitForDeploymentReady(testContext, testManager.GetClient(), "radix-system", "radix-webhook"); err != nil {
		log.Fatal().Err(err).Msg("failed to wait for webhook to be ready")
	}

	// Run tests
	code := m.Run()

	// Cleanup cluster
	if testCluster != nil {
		_ = testCluster.Delete(context.Background())
	}

	if cfg.RemoveImagesOnFinish {
		for _, spec := range componentSpecs {
			_ = internal.RemoveImage(context.Background(), spec.ImageName, imageTag)
		}
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
