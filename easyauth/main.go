/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/equinor/radix-operator/easyauth/internal/controller"
	"github.com/equinor/radix-operator/easyauth/internal/defaulter"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/equinor/radix-operator/pkg/apis/utils/configcodec"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	siglog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var runtimeScheme = scheme.NewScheme()

func main() {
	cfg := loadConfig(context.Background())
	logger := initLogger(cfg)
	logger.Info().Str("version", "dev").Msg("Starting Radix EasyAuth")
	logger.Info().Interface("config", cfg).Msg("Configuration")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Logger: initLogr(logger),
		Scheme: runtimeScheme,
		Metrics: metricsserver.Options{
			BindAddress: fmt.Sprintf(":%d", cfg.MetricsPort),
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    cfg.Port,
			CertDir: cfg.CertsDir,
		}),
		HealthProbeBindAddress: fmt.Sprintf(":%d", cfg.HealthPort),

		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
		LeaderElection:                cfg.EnableLeaderElection,
		LeaderElectionID:              cfg.LeaderId,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start manager") // Exits
	}

	// Register the custom mutating webhook
	err = defaulter.RegisterWithManager(mgr)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to create webhook")
	}

	err = (&controller.RadixApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: runtimeScheme,
	}).SetupWithManager(mgr)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to register controller with manager")
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Fatal().Err(err).Msg("Failed to set up health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Fatal().Err(err).Msg("Failed to set up ready check")
	}

	logger.Info().Msg("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run manager")
	}
	logger.Info().Msg("Manager stopped")
}

func loadConfig(ctx context.Context) EasyAuthConfig {
	cfgClient, err := client.New(k8sconfig.GetConfigOrDie(), client.Options{Scheme: runtimeScheme})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create config reader client")
	}
	cfgYaml := config.MustEnvConfigMapReader(ctx, cfgClient)
	var cfg EasyAuthConfig
	configcodec.MustDecode(cfgYaml, &cfg)
	return cfg
}

func initLogger(cfg EasyAuthConfig) zerolog.Logger {
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
