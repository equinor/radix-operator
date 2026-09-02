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
	ClusterName string            `json:"clusterName" required:"true"`
	OAuth2Proxy OAuth2ProxyConfig `json:"oauth2Proxy"`
}
type OperatorConfig struct {
	LogLevel       string `json:"logLevel"`
	LogPrettyPrint bool   `json:"logPrettyPrint"`

	RegistrationControllerThreads int     `json:"registrationControllerThreads" required:"true"`
	ApplicationControllerThreads  int     `json:"applicationControllerThreads" required:"true"`
	EnvironmentControllerThreads  int     `json:"environmentControllerThreads" required:"true"`
	DeploymentControllerThreads   int     `json:"deploymentControllerThreads" required:"true"`
	JobControllerThreads          int     `json:"jobControllerThreads" required:"true"`
	AlertControllerThreads        int     `json:"alertControllerThreads" required:"true"`
	KubeClientRateLimitBurst      int     `json:"kubeClientRateLimitBurst" required:"true"`
	KubeClientRateLimitQPS        float32 `json:"kubeClientRateLimitQPS" required:"true"`

	ReadinessProbeInitialDelaySeconds int32 `json:"readinessProbeInitialDelaySeconds" required:"true"`

	//RADIXOPERATOR_APP_READINESS_PROBE_PERIOD_SECONDS
	ReadinessProbePeriodSeconds int32 `json:"readinessProbePeriodSeconds" required:"true"`
}

type OAuth2ProxyConfig struct {
	DefaultOIDCIssuer string         `json:"defaultOIDCIssuer"`
	ProxyImage        ContainerImage `json:"proxyImage" required:"true"`
	RedisImage        ContainerImage `json:"redisImage" required:"true"`
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
	if err := processEnvOverrides(&cfg, "RADIX"); err != nil {
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

		if required && value.IsZero() {
			return fmt.Errorf("field %q is required but not set", path)
		}
		return nil
	})
}

func processEnvOverrides(cfg *Config, prefix string) error {
	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {

		envTag := strings.ReplaceAll(path, ".", "_")
		envName := strings.ToUpper(prefix + "_" + envTag)

		envValue := os.Getenv(envName)
		if envValue == "" {
			return nil
		}

		values := []string{envValue}
		if field.Type.Kind() == reflect.Slice {
			values = strings.Split(envValue, ",")
		}

		if err := setter(values...); err != nil {
			return fmt.Errorf("failed to set field %q from env %q: %w", path, envName, err)
		}
		return nil
	})
}
