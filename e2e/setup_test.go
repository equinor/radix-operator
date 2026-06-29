package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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
	{
		Name:         "radix-api-server",
		Dockerfile:   "api-server.Dockerfile",
		ImageName:    "local-kind-repo/api-server",
		HelmValueKey: "radixApiServer.image",
	},
}

type Config struct {
	SetupParallelism uint `envconfig:"E2E_SETUP_PARALLELISM" default:"0" desc:"Limits the number of active goroutines for building images and setting up kind cluster. Value 0 indicates no limit."`
	CleanupOnFinish  bool `envconfig:"E2E_CLEANUP_ON_FINISH" default:"false" desc:"Cleanup resources after test finish."`
}

// TestMain is the entry point for e2e tests
func TestMain(m *testing.M) {
	// Initialize logger for controller-runtime
	logger := initLogger()
	logrLogger := initLogr(logger)

	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		_ = envconfig.Usage("", &cfg)
		log.Fatal().Err(err).Msg("failed to parse process config")
	}
	log.Info().Interface("config", cfg).Msgf("config loaded")

	// Create a context with timeout for the entire test suite
	testContext, testCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer testCancel()

	// Generate image tag
	imageTag := internal.GenerateImageTag()
	log.Info().Msgf("Starting parallel cluster creation and image builds with tag: %s", imageTag)
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

	// Build the in-cluster git server image used as a stand-in for github.com.
	eg.Go(func() error {
		return internal.BuildGitServerImage(testContext, imageTag)
	})

	// Wait for both to complete
	err = eg.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup cluster or build images")
	}
	log.Info().Msgf("Cluster and images ready")

	// Get kubeconfig
	kubeConfig, err := testCluster.GetKubeConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get kubeconfig")
	}

	// Load images into Kind cluster
	log.Info().Msgf("Loading images into Kind cluster...")
	for _, spec := range componentSpecs {
		if err = testCluster.LoadImage(testContext, spec.ImageName, imageTag); err != nil {
			log.Fatal().Err(err).Msg("failed to load image")
		}
	}
	if err = testCluster.LoadImage(testContext, internal.GitServerImageName, imageTag); err != nil {
		log.Fatal().Err(err).Msg("failed to load git server image")
	}

	// Create scheme with all required types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))
	utilruntime.Must(gatewayapiv1.Install(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	// Remove any Kubernetes resources left behind by earlier test runs before (re)installing the
	// Helm chart. This is relevant because the kind cluster may be reused between runs. Deleting the
	// Radix CRDs cascades deletion of all Radix custom resources; the CRDs are reinstalled by the
	// Helm chart below. Uses a direct (non-cached) client.
	cleanupClient, err := client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cleanup client")
	}
	log.Info().Msgf("Cleaning up resources from previous test runs...")
	// Uninstall the radix-operator release first so the running operator does not fail when its
	// CRDs are removed below.
	if err := internal.UninstallRadixOperator(testContext, testCluster.KubeConfigPath, "radix-system", "radix-operator"); err != nil {
		log.Fatal().Err(err).Msg("failed to uninstall radix-operator helm release")
	}
	if err := cleanupClusterResources(testContext, cleanupClient); err != nil {
		log.Fatal().Err(err).Msg("failed to clean up cluster resources")
	}
	log.Info().Msgf("Cluster is clean")

	// Label the cluster nodes so that Radix jobs (which require a node carrying the
	// kube.RadixJobNodeLabel via node affinity) can be scheduled. On a real cluster this label is
	// applied to the dedicated jobs node pool; the single-node kind cluster has no such pool, so we
	// add the label to every node here.
	if err := labelNodesForRadixJobs(testContext, cleanupClient); err != nil {
		log.Fatal().Err(err).Msg("failed to label nodes for radix jobs")
	}

	// Deploy an in-cluster SSH git server as a stand-in for github.com and rewrite the github.com
	// host to it via CoreDNS, so pipeline jobs can clone the application config without reaching
	// the real GitHub. The returned known_hosts entry is used by the git deploy-key secret below.
	log.Info().Msgf("Deploying in-cluster git server...")
	gitKnownHosts, err := internal.DeployGitServer(testContext, cleanupClient, imageTag)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to deploy git server")
	}
	if err := internal.ConfigureCoreDNSGithubRewrite(testContext, cleanupClient); err != nil {
		log.Fatal().Err(err).Msg("failed to configure CoreDNS github rewrite")
	}

	// Create manager
	if testManager, err = manager.New(kubeConfig, manager.Options{
		Scheme: scheme,
		Logger: logrLogger,
		// Disable the metrics server; the tests don't need it and binding the default :8080
		// address conflicts with other processes (or a previous run).
		Metrics: metricsserver.Options{BindAddress: "0"},
	}); err != nil {
		log.Fatal().Err(err).Msg("failed to create manager")
	}

	// Install Prometheus Operator CRDs first
	if err = internal.InstallPrometheusOperatorCRDs(testContext, testCluster.KubeConfigPath); err != nil {
		log.Fatal().Err(err).Msg("failed to install Prometheus Operator CRDs")
	}

	if err = internal.InstallGatewayApiCRDs(testContext, testCluster.KubeConfigPath); err != nil {
		log.Fatal().Err(err).Msg("failed to install Gateway API CRDs")
	}

	// Install Helm chart with custom image tags
	helmValues := map[string]string{
		"clusterName":                           "weekly-e2e",
		"rbac.createApp.groups[0]":              "123",
		"image.pullPolicy":                      "IfNotPresent",
		"radixPipelineRunner.image.pullPolicy":  "IfNotPresent",
		"ingress.gateway.name":                  "test-gateway",
		"ingress.gateway.namespace":             "test-gateway-namespace",
		"radixApiServer.logLevel":               "debug",
		"radixApiServer.logPretty":              "true",
		"radixApiServer.oidcKubernetesIssuer":   "https://kubernetes.default.svc",
		"radixApiServer.oidcKubernetesAudience": "unknown",
		"radixApiServer.oidcAzureIssuer":        "https://sts.windows.net/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/",
		"radixApiServer.oidcAzureAudience":      "6dae42f8-4368-4678-94ff-3960e28e3630",
	}
	for _, spec := range componentSpecs {
		helmValues[fmt.Sprintf("%s.repository", spec.HelmValueKey)] = spec.ImageName
		helmValues[fmt.Sprintf("%s.tag", spec.HelmValueKey)] = imageTag
	}
	if err = internal.InstallRadixOperator(testContext, testCluster.KubeConfigPath, "radix-system", "radix-operator", "../charts/radix-operator", helmValues); err != nil {
		log.Fatal().Err(err).Msg("failed to install radix-operator helm chart")
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

	// Create the cluster prerequisites the operator expects to exist (normally created during
	// Radix bootstrap): the radix-config ConfigMap and the radix-known-hosts-git Secret in the
	// default namespace. Without these the job- and registration-controllers fail to sync.
	if err := createClusterPrerequisites(testContext, testManager.GetClient(), gitKnownHosts); err != nil {
		log.Fatal().Err(err).Msg("failed to create cluster prerequisites")
	}

	// Run tests
	code := m.Run()

	// Cleanup cluster
	if cfg.CleanupOnFinish {
		if testCluster != nil && !testCluster.Existed {
			_ = testCluster.Delete(context.Background())
		}
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
	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
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

// WaitForDeploymentReady waits for a deployment's rollout to be complete and stable using the
// controller-runtime client. It requires the latest generation to be observed and all replicas to
// be updated, available and ready, so it does not return prematurely while the webhook restarts
// once to configure its certificates.
func WaitForDeploymentReady(ctx context.Context, c client.Client, namespace, name string) error {
	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		deployment := &appsv1.Deployment{}
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment)
		if err != nil {
			return false, nil // Deployment doesn't exist yet, keep waiting
		}

		// Desired number of replicas (defaults to 1 when unset)
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}

		status := deployment.Status
		// The rollout is complete and stable when the latest generation has been observed and every
		// replica is updated, available and ready, with no leftover (restarting) replicas.
		if status.ObservedGeneration >= deployment.Generation &&
			desired > 0 &&
			status.UpdatedReplicas == desired &&
			status.Replicas == desired &&
			status.AvailableReplicas == desired &&
			status.ReadyReplicas == desired {
			return true, nil
		}

		return false, nil
	})
}

// labelNodesForRadixJobs adds the kube.RadixJobNodeLabel label to every node in the cluster so that
// Radix job pods, whose node affinity requires a node carrying that label, can be scheduled on the
// single-node kind cluster (which has no dedicated jobs node pool). It is idempotent.
func labelNodesForRadixJobs(ctx context.Context, c client.Client) error {
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if _, ok := node.Labels[kube.RadixJobNodeLabel]; ok {
			continue
		}
		patch := client.MergeFrom(node.DeepCopy())
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels[kube.RadixJobNodeLabel] = "true"
		if err := c.Patch(ctx, node, patch); err != nil {
			return fmt.Errorf("failed to label node %s: %w", node.Name, err)
		}
	}

	return nil
}

// createClusterPrerequisites creates the cluster-wide resources the operator expects to exist,
// which are normally provisioned during Radix bootstrap: the radix-config ConfigMap (cluster name
// and subscription id) and the radix-known-hosts-git Secret, both in the default namespace. These
// are required by the job- and registration-controllers. The function is idempotent.
func createClusterPrerequisites(ctx context.Context, c client.Client, gitKnownHosts string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-config",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"clustername":    "weekly-e2e",
			"subscriptionId": "00000000-0000-0000-0000-000000000000",
		},
	}
	if err := c.Create(ctx, configMap); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create radix-config config map: %w", err)
	}

	knownHostsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-known-hosts-git",
			Namespace: corev1.NamespaceDefault,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"known_hosts": []byte(gitKnownHosts),
		},
	}
	// Upsert: the cluster is reused across runs, so an existing secret may hold a stale
	// known_hosts value. The operator copies this verbatim into each app namespace's
	// git-ssh-keys secret, so it must always match the current in-cluster git server host key.
	if err := c.Create(ctx, knownHostsSecret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create radix-known-hosts-git secret: %w", err)
		}
		existing := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(knownHostsSecret), existing); err != nil {
			return fmt.Errorf("failed to get radix-known-hosts-git secret: %w", err)
		}
		existing.Data = knownHostsSecret.Data
		if err := c.Update(ctx, existing); err != nil {
			return fmt.Errorf("failed to update radix-known-hosts-git secret: %w", err)
		}
	}

	// ACR service principal secrets, normally provisioned during Radix bootstrap. The
	// registration-controller copies these into each app namespace, so they must exist in the
	// default namespace.
	acrSecretNames := []string{
		defaults.AzureACRServicePrincipleSecretName,
		defaults.AzureACRServicePrincipleBuildahSecretName,
		defaults.AzureACRTokenPasswordAppRegistrySecretName,
	}
	for _, name := range acrSecretNames {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: corev1.NamespaceDefault,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"sp_credentials.json": "{}",
			},
		}
		if err := c.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %s secret: %w", name, err)
		}
	}

	return nil
}

// cleanupClusterResources removes Radix resources that may have been left behind by earlier
// test runs (relevant when the kind cluster is reused) and blocks until the cluster is clean.
// It deletes all Radix CRDs - which cascades deletion of every Radix custom resource in the
// cluster (RadixRegistration, RadixEnvironment, RadixDNSAlias, RadixApplication, etc.) - and all
// Radix application namespaces. The CRDs are reinstalled afterwards by the Helm chart.
func cleanupClusterResources(ctx context.Context, c client.Client) error {
	// Delete all Radix CRDs. This cascades deletion of every Radix custom resource in the cluster.
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := c.List(ctx, crdList); err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}
	for i := range crdList.Items {
		crd := &crdList.Items[i]
		if crd.Spec.Group != radixv1.GroupName {
			continue
		}
		if err := c.Delete(ctx, crd); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete CRD %s: %w", crd.Name, err)
		}
	}

	// Wait until all Radix CRDs (and therefore all Radix custom resources) are gone
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		list := &apiextensionsv1.CustomResourceDefinitionList{}
		if err := c.List(ctx, list); err != nil {
			return false, nil
		}
		for i := range list.Items {
			if list.Items[i].Spec.Group == radixv1.GroupName {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("failed waiting for Radix CRDs to be removed: %w", err)
	}

	// Delete the Radix webhook configuration. It is created as a Helm post-install hook and is
	// therefore NOT removed by `helm uninstall`. A stale configuration would validate resources
	// (e.g. the api-server HTTPRoute) created during the next Helm install against a webhook whose
	// pod is not yet serving, causing the install to fail with "connection refused".
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "radix-webhook-configuration"},
	}
	if err := c.Delete(ctx, webhookConfig); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete validating webhook configuration: %w", err)
	}

	// Delete all Radix application namespaces (identified by the radix-app label)
	nsList := &corev1.NamespaceList{}
	if err := c.List(ctx, nsList, client.HasLabels{kube.RadixAppLabel}); err != nil {
		return fmt.Errorf("failed to list Radix namespaces: %w", err)
	}
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		if err := c.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace %s: %w", ns.Name, err)
		}
	}

	// Wait until all Radix application namespaces are fully removed before continuing
	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		list := &corev1.NamespaceList{}
		if err := c.List(ctx, list, client.HasLabels{kube.RadixAppLabel}); err != nil || len(list.Items) > 0 {
			return false, nil
		}
		return true, nil
	})
}
