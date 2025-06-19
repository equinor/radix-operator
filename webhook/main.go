package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/types"

	internalconfig "github.com/equinor/radix-operator/webhook/internal/config"
	"github.com/equinor/radix-operator/webhook/internal/handler"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	ctx := signals.SetupSignalHandler()
	c := internalconfig.MustParseConfig()
	logger := initLogger(c)
	logr := initLogr(logger)

	logger.Info().Msg("setting up manager")
	restConfig := config.GetConfigOrDie()
	mgr, err := manager.New(restConfig, manager.Options{Logger: logr})
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up overall controller manager")
	}

	if err := radixv1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal().Err(err).Msg("unable to add radixv1 scheme")
	}

	// Make sure certs are generated and valid if cert rotation is enabled.
	logger.Info().Msg("setting up cert rotation")
	certSetupFinished := addCertRotator(mgr, c)

	client, err := radixclient.NewForConfig(restConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up radix client")
	}

	go func() {
		logger.Debug().Msg("waiting for cert rotation to finish")
		if err := setupWebhook(mgr, client, certSetupFinished); err != nil {
			logger.Fatal().Err(err).Msg("unable to set up webhook")
		}
		logger.Info().Msg("webhook setup complete")
	}()

	logger.Info().Msg("starting manager")
	if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("unable to run manager")
	}
	logger.Info().Msg("shutting down")
}

func setupWebhook(mgr manager.Manager, client radixclient.Interface, certSetupFinished <-chan struct{}) error {
	<-certSetupFinished

	rrValidator := handler.NewRadixRegistrationValidator(client)
	err := builder.WebhookManagedBy(mgr).
		For(&radixv1.RadixRegistration{}).
		WithCustomPath("/api/v1/radixregistration/validator").
		WithValidator(rrValidator).
		Complete()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create webhook")
	}
	return nil
}

func addCertRotator(mgr manager.Manager, c internalconfig.Config) <-chan struct{} {
	setupFinished := make(chan struct{})
	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: c.SecretNamespace,
			Name:      c.SecretName,
		},
		CAName:         c.CaName,
		CAOrganization: c.CaOrganization,
		DNSName:        c.DnsName,
		ExtraDNSNames:  c.ExtraDnsNames,
		IsReady:        setupFinished,
		Webhooks: []rotator.WebhookInfo{
			{
				Name: c.WebhookServiceName,
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

	return setupFinished
}

func initLogr(logger zerolog.Logger) logr.Logger {
	zerologr.NameFieldName = "logger"
	zerologr.NameSeparator = "/"
	zerologr.SetMaxV(1)

	var log logr.Logger = zerologr.New(&logger)

	return log
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
