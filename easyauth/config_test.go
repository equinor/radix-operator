package main

import (
	_ "embed"
	"testing"

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
		EnableLeaderElection: true,
		LeaderId:             "radix-easyauth.easyauth.radix.equinor.com",
		LogLevel:             "debug",
		LogPrettyPrint:       true,
		Port:                 9443,
		MetricsPort:          9000,
		HealthPort:           9440,
		CertsDir:             "/run/certs",
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
