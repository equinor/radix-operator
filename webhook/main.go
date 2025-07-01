package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	internalconfig "github.com/equinor/radix-operator/webhook/internal/config"
	"github.com/equinor/radix-operator/webhook/validator"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/rs/zerolog/log"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func main() {
	ctx := signals.SetupSignalHandler()
	c := internalconfig.MustParseConfig()
	logger := initLogger(c)
	logr := initLogr(logger)
	logger.Info().Str("version", internalconfig.Version).Msg("Starting Radix Webhook")
	logger.Info().Interface("config", c).Msg("Configuration")

	logger.Info().Msg("setting up manager")
	restConfig := config.GetConfigOrDie()

	client, err := radixclient.NewForConfig(restConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up radix client")
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(radixv1.AddToScheme(scheme))
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:                 scheme,
		Logger:                 logr,
		LeaderElection:         false,
		HealthProbeBindAddress: fmt.Sprintf(":%d", c.HealthPort),
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    c.Port,
			CertDir: c.CertDir,
		}),
		Metrics: server.Options{
			BindAddress: fmt.Sprintf(":%d", c.MetricsPort),
		},
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up overall controller manager")
	}

	certSetupFinished := addCertRotator(mgr, c)
	addProbeEndpoints(mgr, certSetupFinished)
	go setupWebhook(ctx, mgr, client, c, certSetupFinished) // blocks until cert rotation is finished (requires manager to start)

	logger.Info().Msg("starting manager")
	if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("unable to run manager")
	}
	logger.Info().Msg("shutting down")
}

func setupWebhook(ctx context.Context, mgr manager.Manager, client radixclient.Interface, c internalconfig.Config, certSetupFinished <-chan struct{}) {
	log.Debug().Msg("Configuring webhook...")
	select {
	case <-certSetupFinished:
	case <-time.NewTicker(1 * time.Minute).C:
		log.Fatal().Msg("Failed to set up certificate rotation before deadline (60sec)")
	}

	rrValidator := validator.NewRadixRegistrationValidator(ctx, client, c.RequireAdGroups, c.RequireConfigurationItem)
	mgr.GetWebhookServer().Register(validator.RadixRegistrationValidatorWebhookPath, admission.WithCustomValidator(mgr.GetScheme(), &radixv1.RadixRegistration{}, rrValidator))

	log.Info().Msg("webhook setup complete")
}

func addCertRotator(mgr manager.Manager, c internalconfig.Config) <-chan struct{} {
	log.Info().Msg("setting up cert rotation")
	setupFinished := make(chan struct{})

	if !c.DisableCertRotation {
		err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: c.SecretNamespace,
				Name:      c.SecretName,
			},
			CAName:                 c.CaName,
			CAOrganization:         c.CaOrganization,
			CertDir:                c.CertDir,
			RestartOnSecretRefresh: true,
			DNSName:                c.DnsName,
			ExtraDNSNames:          c.ExtraDnsNames,
			IsReady:                setupFinished,
			RequireLeaderElection:  false,
			EnableReadinessCheck:   true,
			Webhooks: []rotator.WebhookInfo{
				{
					Name: c.WebhookConfigurationName,
					Type: rotator.Validating,
				},
			},
		})
		if err != nil {
			log.Fatal().Err(err).Msg("unable to set up cert rotation")
		}

		go func() {
			<-setupFinished
			log.Info().Msg("cert rotation setup complete")
		}()
	} else {
		log.Info().Msg("cert rotation disabled, skipping setup")
		close(setupFinished)
	}

	return setupFinished
}

func addProbeEndpoints(mgr manager.Manager, certSetupFinished <-chan struct{}) {
	// Block readiness on the mutating webhook being registered.
	// We can't use mgr.GetWebhookServer().StartedChecker() yet,
	// because that starts the webhook. But we also can't call AddReadyzCheck
	// after Manager.Start. So we need a custom ready check that delegates to
	// the real ready check after the cert has been injected and validator started.
	checker := func(req *http.Request) error {
		select {
		case <-certSetupFinished:
			return mgr.GetWebhookServer().StartedChecker()(req)
		default:
			return fmt.Errorf("certs are not ready yet")
		}
	}

	if err := mgr.AddHealthzCheck("healthz", checker); err != nil {
		panic(fmt.Errorf("unable to add healthz check: %w", err))
	}
	if err := mgr.AddReadyzCheck("readyz", checker); err != nil {
		panic(fmt.Errorf("unable to add readyz check: %w", err))
	}
	mgr.GetLogger().Info("added healthz and readyz check")
}

func initLogger(cfg internalconfig.Config) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
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
	if cfg.LogPrettyPrint {
		logWriter = &zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}

	logger := zerolog.New(logWriter).Level(logLevel).With().Timestamp().Logger()

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger
	return logger
}

func initLogr(logger zerolog.Logger) logr.Logger {
	zerologr.NameFieldName = "logger"
	zerologr.NameSeparator = "/"
	zerologr.SetMaxV(2)

	var log logr.Logger = zerologr.New(&logger)
	siglog.SetLogger(log)

	return log
}
