package config

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

type Validator interface {
	Validate() error
}

var envMacroJSONRegexp = regexp.MustCompile(`"\$__env\(([^)]+)\)"`)

type Config struct {
	Operator       OperatorConfig       `json:"operator"`
	PipelineRunner PipelineRunnerConfig `json:"pipelineRunner"`
	Common         CommonConfig         `json:"common"`
	Webhook        WebhookConfig        `json:"webhook"`
	ApiServer      ApiServerConfig      `json:"apiServer"`
}

type CommonConfig struct {
	DNSZone                    string            `json:"dnsZone" required:"true"`
	ClusterName                string            `json:"clusterName" required:"true"`
	ClusterType                string            `json:"clusterType" required:"true"`
	AppAliasBaseURL            string            `json:"appAliasBaseURL" required:"true"`
	ExternalRegistryAuthSecret string            `json:"externalRegistryAuthSecret"`
	OAuth2Proxy                OAuth2ProxyConfig `json:"oauth2Proxy"`
}

type PipelineRunnerConfig struct {
	ContainerRegistry      string         `json:"containerRegistry" required:"true"`
	CacheContainerRegistry string         `json:"cacheContainerRegistry" required:"true"`
	Builder                BuilderConfig  `json:"builder" required:"true"`
	GitCloneImage          ContainerImage `json:"gitCloneImage" required:"true"`
}

type BuilderConfig struct {
	Image                          ContainerImage `json:"image" required:"true"`
	SeccompProfileLocalhostProfile string         `json:"seccompProfileLocalhostProfile" required:"true"`
	Resources                      Resources      `json:"resources" required:"true" validate:"compareQuantity(self.limits.memory, self.requests.memory) >= 0 && compareQuantity(self.limits.cpu, self.requests.cpu) >= 0"`
}

type OAuth2ProxyConfig struct {
	ProxyImage    ContainerImage `json:"proxyImage" required:"true"`
	RedisImage    ContainerImage `json:"redisImage" required:"true"`
	ProxyDefaults v1.OAuth2      `json:"proxyDefaults"`
}

type LimitRangeConfig struct {
	DefaultMemory        *resource.Quantity `json:"defaultMemory" required:"true"`
	DefaultRequestMemory *resource.Quantity `json:"defaultRequestMemory" required:"true"`
	DefaultRequestCPU    *resource.Quantity `json:"defaultRequestCPU" required:"true"`
}

type Resources struct {
	Requests ResourceRequirements `json:"requests" required:"true"`
	Limits   ResourceRequirements `json:"limits" required:"true"`
}
type ResourceRequirements struct {
	Memory *resource.Quantity `json:"memory" required:"true"`
	CPU    *resource.Quantity `json:"cpu" required:"true"`
}

type PodSecurityStandardConfig struct {
	AppNamespace PodSecurityStandardPolicyConfig `json:"appNamespace"`
	EnvNamespace PodSecurityStandardPolicyConfig `json:"envNamespace"`
}

type PodSecurityStandardPolicyConfig struct {
	Enforce PodSecurityStandardModeConfig `json:"enforce"`
	Audit   PodSecurityStandardModeConfig `json:"audit"`
	Warn    PodSecurityStandardModeConfig `json:"warn"`
}

type PodSecurityStandardModeConfig struct {
	Level   string `json:"level"`
	Version string `json:"version"`
}

func Parse(configYaml string) (*Config, error) {
	var cfg Config
	configJson, err := yaml.YAMLToJSON([]byte(configYaml))
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}
	configJson = expandEnvMacros(configJson)

	if err := json.Unmarshal(configJson, &cfg, Unmarshalers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Parse env overrides
	if err := processEnvOverrides(&cfg, "RADIXCONFIG"); err != nil {
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

func expandEnvMacros(configJson []byte) []byte {
	return envMacroJSONRegexp.ReplaceAllFunc(configJson, func(macro []byte) []byte {
		envName := string(macro[len(`"$__env(`) : len(macro)-2])
		envValue := os.Getenv(envName)
		if envValue == "" {
			return macro
		}
		return []byte(envValue)
	})
}

func validateConfig(cfg *Config) error {
	validator, err := NewCelValidator()
	if err != nil {
		return fmt.Errorf("failed to create config validator: %w", err)
	}

	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, value reflect.Value, _ processfields.SetValFunc) error {
		requiredTag := field.Tag.Get("required")
		required, _ := strconv.ParseBool(requiredTag)

		if value.IsZero() {
			if required {
				return fmt.Errorf("field %q is required but not set", path)
			}

			return nil
		}

		if val, ok := value.Interface().(Validator); ok {
			if err := val.Validate(); err != nil {
				return fmt.Errorf("field %q validation failed: %w", path, err)
			}
		}

		expression := field.Tag.Get("validate")
		if expression != "" {
			if valid, err := validator.ValidateField(expression, cfg, value); err != nil {
				return fmt.Errorf("field %q validation failed expression: %w", path, err)
			} else if !valid {
				return fmt.Errorf("field %q did not pass validation expression", path)
			}
		}

		return nil
	})
}

func processEnvOverrides(cfg *Config, prefix string) error {
	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, val reflect.Value, setter processfields.SetValFunc) error {
		env := strings.ReplaceAll(path, ".", "_")
		env = strings.ReplaceAll(env, "[", "_")
		env = strings.ReplaceAll(env, "]", "_")
		env = strings.ToUpper(prefix + "_" + env)
		env = strings.ReplaceAll(env, "__", "_")
		env = strings.TrimSuffix(env, "_")
		env = strings.TrimPrefix(env, "_")

		envValue := os.Getenv(env)
		if envValue == "" {
			return nil
		}

		if setter == nil {
			return fmt.Errorf("it's not allowed to use env-overrides (%s) on a struct on path %s", env, path)
		}

		values := []string{envValue}
		if field.Type.Kind() == reflect.Slice {
			values = strings.Split(envValue, ",")
		}

		if err := setter(values...); err != nil {
			return fmt.Errorf("failed to set field %q from env %q: %w", path, env, err)
		}
		return nil
	})
}
