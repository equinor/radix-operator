package config

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

type Validator interface {
	Validate() error
}

var envMacroJSONRegexp = regexp.MustCompile(`\$__env\(([^)]+)\)`)

type Config struct {
	Operator       OperatorConfig       `json:"operator"`
	PipelineRunner PipelineRunnerConfig `json:"pipelineRunner"`
	Common         CommonConfig         `json:"common"`
	Webhook        WebhookConfig        `json:"webhook"`
	ApiServer      ApiServerConfig      `json:"apiServer"`
}

func Parse(configYaml string) (*Config, error) {
	expandedConfigYaml := expandEnvMacros([]byte(configYaml))
	var cfg Config
	configJson, err := yaml.YAMLToJSON(expandedConfigYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

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
		envName := string(macro[len(`$__env(`) : len(macro)-1])
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
