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
	c := MustParseConfig()
	logger := initLogger(c)
	logr := initLogr(logger)

	logger.Info().Msg("setting up manager")
	restConfig := config.GetConfigOrDie()
	mgr, err := manager.New(restConfig, manager.Options{Logger: logr})
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up overall controller manager")
	}

	// Make sure certs are generated and valid if cert rotation is enabled.
	logger.Info().Msg("setting up cert rotation")
	if err = addCertRotator(ctx, mgr, c); err != nil {
		logger.Fatal().Err(err).Msg("failed to configure certificate rotation")
	}

	logger.Info().Msg("cert rotation setup complete")

	client, err := radixclient.NewForConfig(restConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to set up radix client")
	}

	rrValidator := NewRadixRegistrationValidator(client)
	err = builder.WebhookManagedBy(mgr).
		For(&radixv1.RadixRegistration{}).
		WithCustomPath("/api/v1/radixregistration/validator").
		WithValidator(rrValidator).
		Complete()
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to create webhook")
	}

	logger.Info().Msg("starting manager")
	if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("unable to run manager")
	}
	logger.Info().Msg("shutting down")
}

func addCertRotator(ctx context.Context, mgr manager.Manager, c Config) error {
	setupFinished := make(chan struct{})
	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: c.SecretNamespace,
			Name:      c.SecretName,
		},
		CertDir:        c.CertsFolder,
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

	select {
	case <-setupFinished:
		return nil
	case <-ctx.Done():
		return ctx.Err()

	}
}

func initLogr(logger zerolog.Logger) logr.Logger {
	zerologr.NameFieldName = "logger"
	zerologr.NameSeparator = "/"
	zerologr.SetMaxV(1)

	var log logr.Logger = zerologr.New(&logger)

	return log
}

func initLogger(cfg Config) zerolog.Logger {
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
