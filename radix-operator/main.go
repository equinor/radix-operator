package main

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/radix-operator/alert"
	"github.com/equinor/radix-operator/radix-operator/application"
	"github.com/equinor/radix-operator/radix-operator/batch"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/deployment"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/environment"
	"github.com/equinor/radix-operator/radix-operator/job"
	"github.com/equinor/radix-operator/radix-operator/registration"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	resyncPeriod            = 0
	ingressConfigurationMap = "radix-operator-ingress-configmap"
)

func main() {
	cfg := config.NewConfig()
	initLogger(cfg)

	registrationControllerThreads, applicationControllerThreads, environmentControllerThreads, deploymentControllerThreads, jobControllerThreads, alertControllerThreads, kubeClientRateLimitBurst, kubeClientRateLimitQPS, err := getInitParams()
	if err != nil {
		panic(err)
	}
	rateLimitConfig := utils.WithKubernetesClientRateLimiter(flowcontrol.NewTokenBucketRateLimiter(kubeClientRateLimitQPS, kubeClientRateLimitBurst))
	warningHandler := utils.WithKubernetesWarningHandler(utils.ZerologWarningHandlerAdapter(log.Warn))
	client, radixClient, prometheusOperatorClient, secretProviderClient, certClient := utils.GetKubernetesClient(rateLimitConfig, warningHandler)

	activeClusterNameEnvVar := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	log.Info().Msgf("Active cluster name: %v", activeClusterNameEnvVar)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go startMetricsServer(ctx.Done())

	eventRecorder := common.NewEventRecorder("Radix controller", client.CoreV1().Events(""), log.Logger)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, resyncPeriod)
	radixInformerFactory := radixinformers.NewSharedInformerFactory(radixClient, resyncPeriod)
	kubeUtil, _ := kube.NewWithListers(
		client,
		radixClient,
		secretProviderClient,
		kubeInformerFactory,
		radixInformerFactory,
	)
	oauthDefaultConfig := getOAuthDefaultConfig()
	ingressConfiguration, err := loadIngressConfigFromMap(kubeUtil)
	if err != nil {
		panic(fmt.Errorf("failed to load ingress configuration: %v", err))
	}

	wg := &sync.WaitGroup{}
	startController(ctx, wg, createRegistrationController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), registrationControllerThreads)
	startController(ctx, wg, createApplicationController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, cfg.DNSConfig), applicationControllerThreads)
	startController(ctx, wg, createEnvironmentController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), environmentControllerThreads)
	startController(ctx, wg, createDeploymentController(kubeUtil, prometheusOperatorClient, certClient, kubeInformerFactory, radixInformerFactory, eventRecorder, oauthDefaultConfig, ingressConfiguration, cfg), deploymentControllerThreads)
	startController(ctx, wg, createJobController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, cfg), jobControllerThreads)
	startController(ctx, wg, createAlertController(kubeUtil, prometheusOperatorClient, kubeInformerFactory, radixInformerFactory, eventRecorder), alertControllerThreads)
	startController(ctx, wg, createBatchController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder), 1)
	startController(ctx, wg, createDNSAliasesController(kubeUtil, kubeInformerFactory, radixInformerFactory, eventRecorder, oauthDefaultConfig, ingressConfiguration, cfg.DNSConfig), environmentControllerThreads)

	// Start informers when all controllers are running
	kubeInformerFactory.Start(ctx.Done())
	radixInformerFactory.Start(ctx.Done())
	<-ctx.Done()
	log.Info().Msg("Shutting down")
	wg.Wait()
	log.Info().Msg("Finished.")
}

func initLogger(cfg *apiconfig.Config) {
	logLevelStr := cfg.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
		log.Warn().Msgf("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	var logWriter io.Writer = os.Stderr
	if cfg.LogPretty {
		logWriter = &zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}

	logger := zerolog.New(logWriter).Level(logLevel).With().Timestamp().Logger()

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
}

func getOAuthDefaultConfig() defaults.OAuth2Config {
	return defaults.NewOAuth2Config(
		defaults.WithOAuth2Defaults(),
		defaults.WithOIDCIssuerURL(os.Getenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)),
	)
}

func startController(ctx context.Context, wg *sync.WaitGroup, controller *common.Controller, threadiness int) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := controller.Run(ctx, threadiness); err != nil {
			log.Fatal().Err(err).Msg("Error running controller")
		}
	}()
}

func getInitParams() (int, int, int, int, int, int, int, float32, error) {
	registrationControllerThreads, regErr := defaults.GetRegistrationControllerThreads()
	applicationControllerThreads, appErr := defaults.GetApplicationControllerThreads()
	environmentControllerThreads, envErr := defaults.GetEnvironmentControllerThreads()
	deploymentControllerThreads, depErr := defaults.GetDeploymentControllerThreads()
	jobControllerThreads, jobErr := defaults.GetJobControllerThreads()
	alertControllerThreads, aleErr := defaults.GetAlertControllerThreads()
	kubeClientRateLimitBurst, burstErr := defaults.GetKubeClientRateLimitBurst()
	kubeClientRateLimitQPS, qpsErr := defaults.GetKubeClientRateLimitQps()

	errCat := stderrors.Join(regErr, appErr, envErr, depErr, jobErr, aleErr, burstErr, qpsErr)
	return registrationControllerThreads, applicationControllerThreads, environmentControllerThreads, deploymentControllerThreads, jobControllerThreads, alertControllerThreads, kubeClientRateLimitBurst, kubeClientRateLimitQPS, errCat
}

func createRegistrationController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := registration.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return registration.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createApplicationController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {
	handler := application.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return application.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createEnvironmentController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := environment.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return environment.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createDNSAliasesController(kubeUtil *kube.Kube,
	kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory,
	recorder record.EventRecorder, oauthDefaultConfig defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration,
	dnsConfig *dnsaliasconfig.DNSConfig) *common.Controller {

	handler := dnsalias.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		dnsConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
		dnsalias.WithIngressConfiguration(ingressConfiguration),
		dnsalias.WithOAuth2DefaultConfig(oauthDefaultConfig),
	)

	return dnsalias.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createDeploymentController(kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, certClient certclient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory,
	recorder record.EventRecorder, oauthDefaultConfig defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration, config *apiconfig.Config) *common.Controller {

	oauth2DockerImage := os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable)
	if oauth2DockerImage == "" {
		panic(fmt.Errorf("failed to read OAuth2 Docker image from environment variable %s", defaults.RadixOAuthProxyImageEnvironmentVariable))
	}
	handler := deployment.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		prometheusOperatorClient,
		certClient,
		config,
		deployment.WithOAuth2DefaultConfig(oauthDefaultConfig),
		deployment.WithIngressConfiguration(ingressConfiguration),
		deployment.WithOAuth2ProxyDockerImage(oauth2DockerImage),
	)

	return deployment.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createJobController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder, config *apiconfig.Config) *common.Controller {
	handler := job.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		config,
		func(syncedOk bool) {}) // Not interested in getting notifications of synced

	return job.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		&handler, kubeInformerFactory, radixInformerFactory, true, recorder)
}

func createAlertController(kubeUtil *kube.Kube, prometheusOperatorClient monitoring.Interface, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := alert.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
		prometheusOperatorClient,
	)

	return alert.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func createBatchController(kubeUtil *kube.Kube, kubeInformerFactory kubeinformers.SharedInformerFactory, radixInformerFactory radixinformers.SharedInformerFactory, recorder record.EventRecorder) *common.Controller {
	handler := batch.NewHandler(
		kubeUtil.KubeClient(),
		kubeUtil,
		kubeUtil.RadixClient(),
	)

	return batch.NewController(
		kubeUtil.KubeClient(),
		kubeUtil.RadixClient(),
		handler,
		kubeInformerFactory,
		radixInformerFactory,
		true,
		recorder)
}

func loadIngressConfigFromMap(kubeutil *kube.Kube) (ingress.IngressConfiguration, error) {
	ingressConfig := ingress.IngressConfiguration{}
	configMap, err := kubeutil.GetConfigMap(metav1.NamespaceDefault, ingressConfigurationMap)
	if err != nil {
		return ingressConfig, err
	}

	err = yaml.Unmarshal([]byte(configMap.Data["ingressConfiguration"]), &ingressConfig)
	if err != nil {
		return ingressConfig, err
	}
	return ingressConfig, nil
}

func startMetricsServer(stop <-chan struct{}) {
	srv := &http.Server{Addr: ":9000"}
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthz", http.HandlerFunc(Healthz))
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start metric server")
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Shutdown metrics server failed")
	}
}

// HealthStatus Represents the data of the endpoint
type HealthStatus struct {
	Status int
}

// Healthz The health endpoint
func Healthz(writer http.ResponseWriter, _ *http.Request) {
	health := HealthStatus{
		Status: http.StatusOK,
	}

	response, err := json.Marshal(health)

	if err != nil {
		http.Error(writer, "Error while retrieving HealthStatus", http.StatusInternalServerError)
		log.Error().Err(err).Msg("Could not serialize HealthStatus")
		return
	}

	_, _ = fmt.Fprintf(writer, "%s", response)
}
