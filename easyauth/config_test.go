package main

import (
	_ "embed"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils/configcodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/config-happypath.yaml
var configHappyYaml []byte

func mutateConfig(t *testing.T, mutate func(*EasyAuthConfig)) []byte {
	t.Helper()
	var cfg EasyAuthConfig
	require.NoError(t, configcodec.Decode(configHappyYaml, &cfg))

	mutate(&cfg)

	configYaml, err := configcodec.Encode(&cfg)
	require.NoError(t, err)
	return configYaml
}

func TestParse_HappyPath(t *testing.T) {
	var cfg EasyAuthConfig
	require.NoError(t, configcodec.Decode(configHappyYaml, &cfg))

	expected := EasyAuthConfig{
		Enforced:             true,
		Annotation:           "radix.equinor.com/easyauth",
		IssuerURL:            "https://simpleauth.dev.radix.equinor.com",
		ClusterName:          "weekly-01",
		DNSZone:              "dev.radix.equinor.com",
		AppAliasBaseURL:      "app.dev.radix.equinor.com",
		EnableLeaderElection: true,
		LeaderId:             "radix-easyauth.easyauth.radix.equinor.com",
		LogLevel:             "debug",
		LogPrettyPrint:       true,
		Port:                 9443,
		MetricsPort:          9000,
		HealthPort:           9440,
		CertsDir:             "/run/certs",
		Dex: DexConfig{
			GRPCAddress: "radix-easyauth-dex.radix-easyauth.svc:5557",
			CertsDir:    "/run/dex-client-certs",
		},
		CleanupEnabled:         true,
		CleanupInterval:        time.Hour,
		SecretRotationInterval: 2160 * time.Hour,
	}
	assert.Equal(t, expected, cfg)
}

func TestParse_Validation(t *testing.T) {
	tests := map[string]struct {
		mutate        func(*EasyAuthConfig)
		expectedError string
	}{
		"leader election enabled with leaderId should pass": {
			mutate: func(cfg *EasyAuthConfig) {
				cfg.EnableLeaderElection = true
				cfg.LeaderId = "any-leader-id"
			},
		},
		"leader election enabled without leaderId should fail": {
			mutate: func(cfg *EasyAuthConfig) {
				cfg.EnableLeaderElection = true
				cfg.LeaderId = ""
			},
			expectedError: `field "EnableLeaderElection" did not pass validation expression`,
		},
		"leader election disabled without leaderId should pass": {
			mutate: func(cfg *EasyAuthConfig) {
				cfg.EnableLeaderElection = false
				cfg.LeaderId = ""
			},
		},
		"missing annotation should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.Annotation = "" },
			expectedError: `field "Annotation" is required but not set`,
		},
		"missing issuerUrl should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.IssuerURL = "" },
			expectedError: `field "IssuerURL" is required but not set`,
		},
		"missing clusterName should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.ClusterName = "" },
			expectedError: `field "ClusterName" is required but not set`,
		},
		"missing dnsZone should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.DNSZone = "" },
			expectedError: `field "DNSZone" is required but not set`,
		},
		"missing appAliasBaseURL should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.AppAliasBaseURL = "" },
			expectedError: `field "AppAliasBaseURL" is required but not set`,
		},
		"missing logLevel should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.LogLevel = "" },
			expectedError: `field "LogLevel" is required but not set`,
		},
		"missing port should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.Port = 0 },
			expectedError: `field "Port" is required but not set`,
		},
		"missing metricsPort should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.MetricsPort = 0 },
			expectedError: `field "MetricsPort" is required but not set`,
		},
		"missing healthPort should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.HealthPort = 0 },
			expectedError: `field "HealthPort" is required but not set`,
		},
		"missing certsDir should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.CertsDir = "" },
			expectedError: `field "CertsDir" is required but not set`,
		},
		"missing dex should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.Dex = DexConfig{} },
			expectedError: `field "Dex" is required but not set`,
		},
		"missing dex grpcAddress should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.Dex.GRPCAddress = "" },
			expectedError: `field "Dex.GRPCAddress" is required but not set`,
		},
		"missing dex certsDir should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.Dex.CertsDir = "" },
			expectedError: `field "Dex.CertsDir" is required but not set`,
		},
		"cleanup enabled without interval should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.CleanupInterval = 0 },
			expectedError: `field "CleanupEnabled" did not pass validation expression`,
		},
		"cleanup enabled with interval below 1m should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.CleanupInterval = 30 * time.Second },
			expectedError: `field "CleanupEnabled" did not pass validation expression`,
		},
		"missing secretRotationInterval should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.SecretRotationInterval = 0 },
			expectedError: `field "SecretRotationInterval" is required but not set`,
		},
		"secretRotationInterval below 1h should fail": {
			mutate:        func(cfg *EasyAuthConfig) { cfg.SecretRotationInterval = 30 * time.Minute },
			expectedError: `field "SecretRotationInterval" did not pass validation expression`,
		},
		"cleanup disabled without interval should pass": {
			mutate: func(cfg *EasyAuthConfig) {
				cfg.CleanupEnabled = false
				cfg.CleanupInterval = 0
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.mutate)

			var cfg EasyAuthConfig
			err := configcodec.Decode(configYaml, &cfg)
			if test.expectedError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorContains(t, err, test.expectedError)
			assert.Empty(t, cfg)
		})
	}
}
