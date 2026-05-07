package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/equinor/radix-operator/api-server/api/alerting"
	"github.com/equinor/radix-operator/api-server/api/applications"
	"github.com/equinor/radix-operator/api-server/api/buildsecrets"
	"github.com/equinor/radix-operator/api-server/api/buildstatus"
	buildModels "github.com/equinor/radix-operator/api-server/api/buildstatus/models"
	"github.com/equinor/radix-operator/api-server/api/configuration"
	"github.com/equinor/radix-operator/api-server/api/deployments"
	"github.com/equinor/radix-operator/api-server/api/environments"
	"github.com/equinor/radix-operator/api-server/api/environmentvariables"
	"github.com/equinor/radix-operator/api-server/api/jobs"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/api/metrics/prometheus"
	"github.com/equinor/radix-operator/api-server/api/privateimagehubs"
	"github.com/equinor/radix-operator/api-server/api/router"
	"github.com/equinor/radix-operator/api-server/api/secrets"
	"github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	token "github.com/equinor/radix-operator/api-server/api/utils/token"
	_ "github.com/equinor/radix-operator/api-server/docs"
	"github.com/equinor/radix-operator/api-server/internal/config"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:generate swagger generate spec
func main() {
	c := config.MustParse()
	setupLogger(c.LogLevel, c.LogPrettyPrint)
	log.Info().Any("config", c).Msgf("Starting radix-api %s in %s environment", c.AppName, c.EnvironmentName)

	servers := []*http.Server{
		initializeServer(c),
		initializeMetricsServer(c),
	}

	if c.UseProfiler {
		log.Info().Msgf("Initializing profile server on port %d", c.ProfilePort)
		servers = append(servers, &http.Server{Addr: fmt.Sprintf("localhost:%d", c.ProfilePort)})
	}

	startServers(servers...)
	shutdownServersGracefulOnSignal(servers...)
}

func initializeServer(c config.Config) *http.Server {
	jwtValidator := initializeTokenValidator(c)
	controllers, err := getControllers(c)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to initialize controllers: %v", err)
	}

	handler := router.NewAPIHandler(jwtValidator, utils.NewKubeUtil(), controllers...)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.Port),
		Handler: handler,
	}

	return srv
}

func initializeTokenValidator(c config.Config) token.ValidatorInterface {
	azureValidator, err := token.NewValidator(c.AzureOidc.Issuer, c.AzureOidc.Audience)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating JWT Azure OIDC validator")
	}

	kubernetesValidator, err := token.NewValidator(c.KubernetesOidc.Issuer, c.KubernetesOidc.Audience)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating JWT Kubernetes OIDC validator")
	}

	chainedValidator := token.NewChainedValidator(azureValidator, kubernetesValidator)
	return chainedValidator
}

func initializeMetricsServer(c config.Config) *http.Server {
	log.Info().Msgf("Initializing metrics server on port %d", c.MetricsPort)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", c.MetricsPort),
		Handler: router.NewMetricsHandler(),
	}
}

func startServers(servers ...*http.Server) {
	for _, srv := range servers {
		srv := srv
		go func() {
			log.Info().Msgf("Starting server on address %s", srv.Addr)
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Fatal().Err(err).Msgf("Unable to start server on address %s", srv.Addr)
			}
		}()
	}
}

func shutdownServersGracefulOnSignal(servers ...*http.Server) {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	s := <-stopCh
	log.Info().Msgf("Received %v signal", s)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	for _, srv := range servers {
		srv := srv
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info().Msgf("Shutting down server on address %s", srv.Addr)
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Warn().Err(err).Msgf("shutdown of server on address %s returned an error", srv.Addr)
			}
		}()
	}

	wg.Wait()
}

func setupLogger(logLevelStr string, prettyPrint bool) {
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
		log.Warn().Msgf("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	var logWriter io.Writer = os.Stderr
	if prettyPrint {
		logWriter = &zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}
	}

	logger := zerolog.New(logWriter).Level(logLevel).With().Timestamp().Logger()

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
}

func getControllers(config config.Config) ([]models.Controller, error) {
	buildStatus := buildModels.NewPipelineBadge()
	applicationFactory := applications.NewApplicationHandlerFactory(config)
	prometheusClient, err := prometheus.NewPrometheusClient(config.PrometheusUrl)
	if err != nil {
		return nil, err
	}
	metricsHandler := metrics.NewHandler(prometheusClient)
	return []models.Controller{
		applications.NewApplicationController(nil, applicationFactory, metricsHandler),
		deployments.NewDeploymentController(),
		jobs.NewJobController(),
		environments.NewEnvironmentController(environments.NewEnvironmentHandlerFactory()),
		environmentvariables.NewEnvVarsController(),
		privateimagehubs.NewPrivateImageHubController(),
		buildsecrets.NewBuildSecretsController(),
		buildstatus.NewBuildStatusController(buildStatus),
		alerting.NewAlertingController(),
		secrets.NewSecretController(tlsvalidation.DefaultValidator()),
		configuration.NewConfigurationController(configuration.Init(config)),
	}, nil
}
