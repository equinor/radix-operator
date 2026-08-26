package config2

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Operator OperatorConfig `json:"operator"`
	Common   CommonConfig   `json:"common"`
}

type CommonConfig struct {
	ClusterName string `json:"clusterName" env:"CLUSTER_NAME" required:"true"`
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

func Parse(ctx context.Context, c client.Client, namespace, name string) (*Config, error) {
	var cfg Config

	cm := &corev1.ConfigMap{Namespace: namespace, Name: name}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
		return nil, fmt.Errorf("failed to load config from cluster: %w", err)
	}

	// Parse config
	configYaml := cm.Data["config"]
	if err := yaml.Unmarshal([]byte(configYaml), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML: %w", err)
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

func MustParse(ctx context.Context, c client.Client, namespace, name string) Config {
	cfg, err := Parse(ctx, c, namespace, name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}
	return *cfg
}

func validateConfig(cfg *Config) error {
	return processFields(cfg, func(field reflect.StructField, value reflect.Value) error {
		requiredTag := field.Tag.Get("required")
		required, _ := strconv.ParseBool(requiredTag)
		if required && value.IsZero() {
			return fmt.Errorf("field %q is required but not set", field.Name)
		}
		return nil
	})
}

func processEnvOverrides(cfg *Config) error {
	return processFields(cfg, func(field reflect.StructField, value reflect.Value) error {
		envTag := field.Tag.Get("env")
		if envTag == "" {
			return nil
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			return nil
		}

		if err := setFieldValue(value, envValue); err != nil {
			return fmt.Errorf("failed to set field %q from env %q: %w", field.Name, envTag, err)
		}
		return nil
	})
}

func setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field %q", field.Type().Name())
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("failed to parse bool: %w", err)
		}
		field.SetBool(boolValue)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	return nil
}

func processFields(cfg any, fn func(field reflect.StructField, value reflect.Value) error) error {
	val := reflect.ValueOf(cfg)
	var typ reflect.Type
	if val.Kind() == reflect.Pointer {
		typ = val.Elem().Type()
	} else {
		typ = val.Type()
	}

	for i := 0; i < typ.NumField(); i++ {
		// pull out the struct tags:
		//    required - whether the field is required
		field := typ.Field(i)
		fieldV := reflect.Indirect(val).Field(i)

		requiredTag := field.Tag.Get("required")
		required, _ := strconv.ParseBool(requiredTag)
		if required && fieldV.IsZero() {
			return fmt.Errorf("field %q is required but not set", field.Name)
		}

		if field.Name == strings.ToLower(field.Name) {
			// Unexported fields cannot be set by a user, so won't have tags or flags, skip them
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			err := processFields(fieldV.Addr().Interface(), fn)
			if err != nil {
				return err
			}
			continue
		}
		err := fn(field, fieldV)
		if err != nil {
			return err
		}
	}

	return nil
}
