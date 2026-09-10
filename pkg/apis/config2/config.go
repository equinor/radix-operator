package config2

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	Operator OperatorConfig `json:"operator"`
	Common   CommonConfig   `json:"common"`
}

type CommonConfig struct {
	DNSZone     string            `json:"dnsZone" required:"true"`
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

	AppAliasBaseURL      string `json:"appAliasBaseURL" required:"true"`
	ContainerRegistry    string `json:"containerRegistry" required:"true"`
	AppContainerRegistry string `json:"appContainerRegistry" required:"true"`

	ClusterType string `json:"clusterType" required:"true"`

	DefaultAppAdminGroups []string `json:"defaultAppAdminGroups"`

	ReadinessProbeInitialDelaySeconds int32 `json:"readinessProbeInitialDelaySeconds" required:"true"`
	ReadinessProbePeriodSeconds       int32 `json:"readinessProbePeriodSeconds" required:"true"`

	DefaultRollingUpdateMaxUnavailable string `json:"defaultRollingUpdateMaxUnavailable" required:"true"`
	DefaultRollingUpdateMaxSurge       string `json:"defaultRollingUpdateMaxSurge" required:"true"`

	AppNsLimitRange LimitRangeConfig `json:"appNsLimitRange" required:"true"`
	EnvNsLimitRange LimitRangeConfig `json:"envNsLimitRange" required:"true"`

	Builder BuilderConfig `json:"builder" required:"true"`

	JobSchedulerImage   ContainerImage            `json:"jobSchedulerImage" required:"true"`
	PodSecurityStandard PodSecurityStandardConfig `json:"podSecurityStandard"`

	// BatchSafeToRestartJobThreshold is the threshold in seconds for determining the cluster-autoscaler safe-to-evict annotation on batch jobs.
	// Jobs with timeLimitSeconds >= BatchSafeToRestartJobThreshold are marked as safe to evict.
	BatchSafeToRestartJobThreshold int64 `json:"batchSafeToRestartJobThreshold" required:"true"`

	// Name of the secret container docker authentication for external registries
	ExternalRegistryAuthSecret string `json:"externalRegistryAuthSecret"`
	AzureKeyVaultTenantID      string `json:"azureKeyVaultTenantID" required:"true"`

	JobSchedulerAuxImage ContainerImage `json:"jobSchedulerAuxImage" required:"true"`

	KubernetesAPIPort      int32                       `json:"kubernetesAPIPort" required:"true"`
	DeploymentHistoryLimit int                         `json:"deploymentHistoryLimit" required:"true" validate:"self >= 3"`
	Gateway                GatewayConfig               `json:"gateway" required:"true"`
	CertificateAutomation  CertificateAutomationConfig `json:"certificateAutomation" required:"true"`
	// OrphanedRadixEnvironmentsRetentionPeriod is the time period for how long orphaned RadixEnvironments should be retained
	OrphanedRadixEnvironmentsRetentionPeriod time.Duration `json:"orphanedEnvironmentsRetentionPeriod" required:"true" validate:"compareDuration(self, '5m') >= 0"`
	// OrphanedEnvironmentsCleanupCron is the cron expression for when to run the cleanup of orphaned RadixEnvironments
	OrphanedEnvironmentsCleanupCron string `json:"orphanedEnvironmentsCleanupCron" required:"true"`

	PipelineJobsHistoryLimit       int           `json:"pipelineJobsHistoryLimit" required:"true" validate:"self >= 3"`
	PipelineJobsHistoryPeriodLimit time.Duration `json:"pipelineJobsHistoryPeriodLimit" required:"true" validate:"compareDuration(self, '24h') >= 0"`
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

// TODO: Probably convert to pod spec defaults instead of just resources, but for now we only need resources
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

	if err := json.Unmarshal(configJson, &cfg, BinaryUnmarshaler, DurationUnmarshaler); err != nil {
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
