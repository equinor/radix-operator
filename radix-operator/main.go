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
	"syscall"
	"time"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
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
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

const (
	resyncPeriod            = 0
	ingressConfigurationMap = "radix-operator-ingress-configmap"
)

type Options struct {
	registrationControllerThreads int
	applicationControllerThreads  int
	environmentControllerThreads  int
	deploymentControllerThreads   int
	jobControllerThreads          int
	alertControllerThreads        int
	kubeClientRateLimitBurst      int
	kubeClientRateLimitQPS        float32
	activeClusterNameEnvVar       string
}

type App struct {
	opts                     Options
	rateLimitConfig          utils.KubernetesClientConfigOption
	warningHandler           utils.KubernetesClientConfigOption
	eventRecorder            record.EventRecorder
	kubeInformerFactory      kubeinformers.SharedInformerFactory
	radixInformerFactory     radixinformers.SharedInformerFactory
	client                   kubernetes.Interface
	radixClient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	secretProviderClient     secretProviderClient.Interface
	certClient               certclient.Interface
	oauthDefaultConfig       defaults.OAuth2Config
	ingressConfiguration     ingress.IngressConfiguration
	kubeUtil                 *kube.Kube
	config                   *apiconfig.Config
	kedaClient               kedav2.Interface
}

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ctx.Done()
		log.Ctx(ctx).Info().Msg("Shutting down gracefully")
	}()

	app, err := initializeApp(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize app")
	}
	log.Ctx(ctx).Info().Msgf("Active cluster name: %v", app.opts.activeClusterNameEnvVar)

	err = app.Run(ctx)
	if err != nil {
		log.Fatal().Msgf(err.Error())
	}

	log.Ctx(ctx).Info().Msg("Finished.")
}

func initializeApp(ctx context.Context) (*App, error) {
	var app App
	var err error

	app.config = config.NewConfig()
	initLogger(app.config)

	app.opts, err = getInitParams()
	if err != nil {
		return nil, fmt.Errorf("failed to get init parameters: %w", err)
	}
	app.rateLimitConfig = utils.WithKubernetesClientRateLimiter(flowcontrol.NewTokenBucketRateLimiter(app.opts.kubeClientRateLimitQPS, app.opts.kubeClientRateLimitBurst))
	app.warningHandler = utils.WithKubernetesWarningHandler(utils.ZerologWarningHandlerAdapter(log.Warn))
	app.client, app.radixClient, app.kedaClient, app.prometheusOperatorClient, app.secretProviderClient, app.certClient = utils.GetKubernetesClient(ctx, app.rateLimitConfig, app.warningHandler)

	app.eventRecorder = common.NewEventRecorder("Radix controller", app.client.CoreV1().Events(""), log.Logger)

	app.kubeInformerFactory = kubeinformers.NewSharedInformerFactory(app.client, resyncPeriod)
	app.radixInformerFactory = radixinformers.NewSharedInformerFactory(app.radixClient, resyncPeriod)
	app.kubeUtil, _ = kube.NewWithListers(
		app.client,
		app.radixClient,
		app.kedaClient,
		app.secretProviderClient,
		app.kubeInformerFactory,
		app.radixInformerFactory,
	)
	app.oauthDefaultConfig = getOAuthDefaultConfig()
	app.ingressConfiguration, err = loadIngressConfigFromMap(ctx, app.kubeUtil)
	if err != nil {
		return nil, fmt.Errorf("failed to load ingress configuration: %w", err)
	}
	return &app, nil
}

func (a *App) Run(ctx context.Context) error {
	var g errgroup.Group
	g.SetLimit(-1)

	registrationController := a.createRegistrationController(ctx)
	jobController := a.createJobController(ctx)
	applicationController := a.createApplicationController(ctx)
	environmentController := a.createEnvironmentController(ctx)
	deploymentController := a.createDeploymentController(ctx)
	alertController := a.createAlertController(ctx)
	batchController := a.createBatchController(ctx)
	dnsAliasesController := a.createDNSAliasesController(ctx)

	g.Go(func() error { return startMetricsServer(ctx) })
	g.Go(func() error { return registrationController.Run(ctx, a.opts.registrationControllerThreads) })
	g.Go(func() error { return applicationController.Run(ctx, a.opts.applicationControllerThreads) })
	g.Go(func() error { return environmentController.Run(ctx, a.opts.environmentControllerThreads) })
	g.Go(func() error { return deploymentController.Run(ctx, a.opts.deploymentControllerThreads) })
	g.Go(func() error { return jobController.Run(ctx, a.opts.jobControllerThreads) })
	g.Go(func() error { return alertController.Run(ctx, a.opts.alertControllerThreads) })
	g.Go(func() error { return batchController.Run(ctx, 1) })
	g.Go(func() error { return dnsAliasesController.Run(ctx, a.opts.environmentControllerThreads) })

	// Informers must be started after all controllers are initialized
	// Therefore we must initialize the controllers outside of go routines
	a.kubeInformerFactory.Start(ctx.Done())
	a.radixInformerFactory.Start(ctx.Done())

	return g.Wait()
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

func getInitParams() (Options, error) {
	registrationControllerThreads, regErr := defaults.GetRegistrationControllerThreads()
	applicationControllerThreads, appErr := defaults.GetApplicationControllerThreads()
	environmentControllerThreads, envErr := defaults.GetEnvironmentControllerThreads()
	deploymentControllerThreads, depErr := defaults.GetDeploymentControllerThreads()
	jobControllerThreads, jobErr := defaults.GetJobControllerThreads()
	alertControllerThreads, aleErr := defaults.GetAlertControllerThreads()
	kubeClientRateLimitBurst, burstErr := defaults.GetKubeClientRateLimitBurst()
	kubeClientRateLimitQPS, qpsErr := defaults.GetKubeClientRateLimitQps()

	return Options{
		registrationControllerThreads: registrationControllerThreads,
		applicationControllerThreads:  applicationControllerThreads,
		environmentControllerThreads:  environmentControllerThreads,
		deploymentControllerThreads:   deploymentControllerThreads,
		jobControllerThreads:          jobControllerThreads,
		alertControllerThreads:        alertControllerThreads,
		kubeClientRateLimitBurst:      kubeClientRateLimitBurst,
		kubeClientRateLimitQPS:        kubeClientRateLimitQPS,
		activeClusterNameEnvVar:       os.Getenv(defaults.ActiveClusternameEnvironmentVariable),
	}, stderrors.Join(regErr, appErr, envErr, depErr, jobErr, aleErr, burstErr, qpsErr)
}

func (a *App) createRegistrationController(ctx context.Context) *common.Controller {
	handler := registration.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return registration.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		&handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createApplicationController(ctx context.Context) *common.Controller {
	handler := application.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		a.config.DNSConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return application.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		&handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createEnvironmentController(ctx context.Context) *common.Controller {
	handler := environment.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
	)

	return environment.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		&handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createDNSAliasesController(ctx context.Context) *common.Controller {

	handler := dnsalias.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		a.config.DNSConfig,
		func(syncedOk bool) {}, // Not interested in getting notifications of synced
		dnsalias.WithIngressConfiguration(a.ingressConfiguration),
		dnsalias.WithOAuth2DefaultConfig(a.oauthDefaultConfig),
	)

	return dnsalias.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createDeploymentController(ctx context.Context) *common.Controller {

	oauth2DockerImage := os.Getenv(defaults.RadixOAuthProxyImageEnvironmentVariable)
	if oauth2DockerImage == "" {
		panic(fmt.Errorf("failed to read OAuth2 Docker image from environment variable %s", defaults.RadixOAuthProxyImageEnvironmentVariable))
	}
	handler := deployment.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		a.kedaClient,
		a.prometheusOperatorClient,
		a.certClient,
		a.config,
		deployment.WithOAuth2DefaultConfig(a.oauthDefaultConfig),
		deployment.WithIngressConfiguration(a.ingressConfiguration),
		deployment.WithOAuth2ProxyDockerImage(oauth2DockerImage),
	)

	return deployment.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createJobController(ctx context.Context) *common.Controller {
	handler := job.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		a.config,
		func(syncedOk bool) {}) // Not interested in getting notifications of synced

	return job.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		handler, a.kubeInformerFactory, a.radixInformerFactory, true, a.eventRecorder)
}

func (a *App) createAlertController(ctx context.Context) *common.Controller {
	handler := alert.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
		a.prometheusOperatorClient,
	)

	return alert.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func (a *App) createBatchController(ctx context.Context) *common.Controller {
	handler := batch.NewHandler(
		a.kubeUtil.KubeClient(),
		a.kubeUtil,
		a.kubeUtil.RadixClient(),
	)

	return batch.NewController(
		ctx,
		a.kubeUtil.KubeClient(),
		a.kubeUtil.RadixClient(),
		handler,
		a.kubeInformerFactory,
		a.radixInformerFactory,
		true,
		a.eventRecorder)
}

func loadIngressConfigFromMap(ctx context.Context, kubeutil *kube.Kube) (ingress.IngressConfiguration, error) {
	ingressConfig := ingress.IngressConfiguration{}
	configMap, err := kubeutil.GetConfigMap(ctx, metav1.NamespaceDefault, ingressConfigurationMap)
	if err != nil {
		return ingressConfig, err
	}

	err = yaml.Unmarshal([]byte(configMap.Data["ingressConfiguration"]), &ingressConfig)
	if err != nil {
		return ingressConfig, err
	}
	return ingressConfig, nil
}

func startMetricsServer(ctx context.Context) error {
	srv := &http.Server{Addr: ":9000"}
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthz", http.HandlerFunc(Healthz))
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Ctx(ctx).Fatal().Err(err).Msg("Failed to start metric server")
		}
		log.Ctx(ctx).Info().Msg("Metrics server closed")
	}()
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Shutdown metrics server failed")
	}
	return nil
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
