package config2

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Operator OperatorConfig `json:"operator"`
	Common   CommonConfig   `json:"common"`
}

type CommonConfig struct {
	ClusterName string            `json:"clusterName" env:"CLUSTER_NAME" required:"true"`
	OAuth2Proxy OAuth2ProxyConfig `json:"oauth2Proxy"`
}
type OperatorConfig struct {
	LogLevel       string `json:"logLevel" env:"OPERATOR_LOG_LEVEL"`
	LogPrettyPrint bool   `json:"logPrettyPrint" env:"OPERATOR_LOG_PRETTY_PRINT"`

	RegistrationControllerThreads int     `json:"registrationControllerThreads" env:"OPERATOR_REGISTRATION_CONTROLLER_THREADS" required:"true"`
	ApplicationControllerThreads  int     `json:"applicationControllerThreads" env:"OPERATOR_APPLICATION_CONTROLLER_THREADS" required:"true"`
	EnvironmentControllerThreads  int     `json:"environmentControllerThreads" env:"OPERATOR_ENVIRONMENT_CONTROLLER_THREADS" required:"true"`
	DeploymentControllerThreads   int     `json:"deploymentControllerThreads" env:"OPERATOR_DEPLOYMENT_CONTROLLER_THREADS" required:"true"`
	JobControllerThreads          int     `json:"jobControllerThreads" env:"OPERATOR_JOB_CONTROLLER_THREADS" required:"true"`
	AlertControllerThreads        int     `json:"alertControllerThreads" env:"OPERATOR_ALERT_CONTROLLER_THREADS" required:"true"`
	KubeClientRateLimitBurst      int     `json:"kubeClientRateLimitBurst" env:"OPERATOR_KUBE_CLIENT_RATE_LIMIT_BURST" required:"true"`
	KubeClientRateLimitQPS        float32 `json:"kubeClientRateLimitQPS" env:"OPERATOR_KUBE_CLIENT_RATE_LIMIT_QPS" required:"true"`
}

type OAuth2ProxyConfig struct {
	DefaultOIDCIssuer string `json:"defaultOIDCIssuer" env:"OAUTH_PROXY_DEFAULT_OIDC_ISSUER_URL"`
}

func Parse(configYaml string) (*Config, error) {
	var cfg Config

	configJson, err := yaml.YAMLToJSON([]byte(configYaml))
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	if err := json.Unmarshal(configJson, &cfg, binaryUnmarshaler); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Parse env overrides
	if err := processEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("failed to process env overrides: %w", err)
	}

	// Validate
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}
	return &cfg, nil
}

func MustParse(configYaml string) Config {
	cfg, err := Parse(configYaml)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}
	return *cfg
}

func validateConfig(cfg *Config) error {
	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, value reflect.Value, _ processfields.SetValFunc) error {
		requiredTag := field.Tag.Get("required")
		required, _ := strconv.ParseBool(requiredTag)

		if required && (value.IsZero() || value.IsNil()) {
			return fmt.Errorf("field %q is required but not set", path)
		}
		return nil
	})
}

func processEnvOverrides(cfg *Config) error {
	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		envTag := field.Tag.Get("env")
		if envTag == "" {
			return nil
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			return nil
		}

		values := []string{envValue}
		if field.Type.Kind() == reflect.Slice {
			values = strings.Split(envValue, ",")
		}

		if err := setter(values...); err != nil {
			return fmt.Errorf("failed to set field %q from env %q: %w", path, envTag, err)
		}
		return nil
	})
}
