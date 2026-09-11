package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/types"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/equinor/radix-operator/webhook/validation"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func main() {
	ctx := signals.SetupSignalHandler()
	cfg := loadConfig(ctx)
	logger := initLogger(cfg)
	logger.Info().Str("version", Version).Msg("Starting Radix Webhook")
	logger.Info().Interface("config", cfg).Msg("Configuration")

	logger.Info().Msg("setting up manager")
	mgr, err := manager.New(k8sconfig.GetConfigOrDie(), manager.Options{
		Scheme:                 scheme.NewScheme(),
		Logger:                 initLogr(logger),
		LeaderElection:         false,
		HealthProbeBindAddress: fmt.Sprintf(":%d", cfg.Webhook.HealthPort),
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    cfg.Webhook.Port,
			CertDir: cfg.Webhook.CertsDir,
		}),
		Metrics: server.Options{
			BindAddress: fmt.Sprintf(":%d", cfg.Webhook.MetricsPort),
		},
	})

	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up overall controller manager")
	}

	certSetupFinished := addCertRotator(mgr, cfg)
	addProbeEndpoints(mgr, certSetupFinished)
	go setupWebhook(mgr, cfg, certSetupFinished) // blocks until cert rotation is finished (requires manager to start)

	logger.Info().Msg("starting manager")
	if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("unable to run manager")
	}
	logger.Info().Msg("shutting down")
}

func loadConfig(ctx context.Context) config.Config {
	cfgClient, err := client.New(k8sconfig.GetConfigOrDie(), client.Options{Scheme: scheme.NewScheme()})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create config reader client")
	}
	cfgYaml := config.MustEnvConfigMapReader(ctx, cfgClient)
	return config.MustParse(cfgYaml)
}

func setupWebhook(mgr manager.Manager, cfg config.Config, certSetupFinished <-chan struct{}) {
	<-certSetupFinished
	log.Debug().Msg("Configuring webhook...")
	validation.SetupWebhook(mgr, cfg)
	log.Info().Msg("webhook setup complete")
}

func addCertRotator(mgr manager.Manager, cfg config.Config) <-chan struct{} {
	log.Info().Msg("setting up cert rotation")
	setupFinished := make(chan struct{})

	if !cfg.Webhook.DisableCertRotation {
		err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: cfg.Webhook.SecretNamespace,
				Name:      cfg.Webhook.SecretName,
			},
			CAName:                 cfg.Webhook.CAName,
			CAOrganization:         cfg.Webhook.CAOrganization,
			CertDir:                cfg.Webhook.CertsDir,
			RestartOnSecretRefresh: true,
			DNSName:                cfg.Webhook.DNSName,
			ExtraDNSNames:          cfg.Webhook.ExtraDNSNames,
			IsReady:                setupFinished,
			RequireLeaderElection:  false,
			EnableReadinessCheck:   true,
			Webhooks: []rotator.WebhookInfo{
				{
					Name: cfg.Webhook.ValidatingWebhookConfigurationName,
					Type: rotator.Validating,
				},
			},
		})
		if err != nil {
			log.Fatal().Err(err).Msg("unable to set up cert rotation")
		}

		go func() {
			select {
			case <-setupFinished:
			case <-time.NewTicker(60 * time.Second).C:
				log.Fatal().Msg("Failed to set up certificate rotation before deadline (60sec)")
			}
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

func initLogger(cfg config.Config) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	logLevelStr := cfg.Webhook.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
		log.Warn().Msgf("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	var logWriter io.Writer = os.Stderr
	if cfg.Webhook.LogPrettyPrint {
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
