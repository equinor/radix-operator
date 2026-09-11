package config_test

import (
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	_ "embed"

	"github.com/equinor/radix-operator/pkg/apis/config"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

//go:embed testdata/config-happypath.yaml
var configHappyYaml string

//go:embed testdata/config-missing-required.yaml
var configMissingRequiredYaml string

type MutateConfigFunc func(*config.Config)

func mutateConfig(t *testing.T, mutate func(*config.Config)) string {
	t.Helper()

	var cfg config.Config

	configJson, err := yaml.YAMLToJSON([]byte(configHappyYaml))
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(configJson, &cfg, config.BinaryUnmarshaler, config.DurationUnmarshaler))

	mutate(&cfg)

	cfgJson, err := json.Marshal(cfg, config.DurationMarshaller)
	require.NoError(t, err)

	configYaml, err := yaml.JSONToYAML(cfgJson)
	require.NoError(t, err)
	return string(configYaml)
}

func TestParse_HappyPath(t *testing.T) {
	cfg, err := config.Parse(configHappyYaml)
	require.NoError(t, err)

	expected := &config.Config{
		Common: config.CommonConfig{
			DNSZone:                    "dev.local.radix.equinor.com",
			ClusterName:                "test-cluster",
			ClusterType:                "development",
			AppAliasBaseURL:            "app.dev.radix.equinor.com",
			ExternalRegistryAuthSecret: "anyExternalAuth",
			OAuth2Proxy: config.OAuth2ProxyConfig{
				ProxyImage: config.ContainerImage{
					Repository: "quay.io/oauth2-proxy/oauth2-proxy",
					Tag:        "v7.6.2",
				},
				RedisImage: config.ContainerImage{
					Repository: "docker.io/redis",
					Tag:        "v8.6.0",
				},
				ProxyDefaults: v1.OAuth2{
					Scope:                  "openid profile email",
					ProxyPrefix:            "/oauth2",
					SetXAuthRequestHeaders: new(false),
					SetAuthorizationHeader: new(false),
					SessionStoreType:       v1.SessionStoreCookie,
					Cookie: &v1.OAuth2Cookie{
						Name:     "_oauth2_proxy",
						Expire:   "168h0m0s",
						Refresh:  "60m0s",
						SameSite: v1.SameSiteLax,
					},
					OIDC: &v1.OAuth2OIDC{
						IssuerURL:     "https://issuer.com",
						SkipDiscovery: new(false),
					},
				},
			},
		},
		PipelineRunner: config.PipelineRunnerConfig{
			ContainerRegistry:      "any.registry.com",
			CacheContainerRegistry: "app.registry.com",
			Builder: config.BuilderConfig{
				Resources: config.Resources{
					Limits: config.ResourceRequirements{
						Memory: new(resource.MustParse("500M")),
						CPU:    new(resource.MustParse("2000m")),
					},
					Requests: config.ResourceRequirements{
						Memory: new(resource.MustParse("500M")),
						CPU:    new(resource.MustParse("200m")),
					},
				},
				Image: config.ContainerImage{
					Repository: "ghcr.io/equinor/radix/buildkit-builder",
					Tag:        "v3.4.5",
				},
				SeccompProfileLocalhostProfile: "anyseccomp.json",
			},
			GitCloneImage: config.ContainerImage{
				Repository: "ghcr.io/equinor/radix-git-clone",
				Tag:        "v1.0.0",
			},
		},
		Operator: config.OperatorConfig{
			LogLevel:                          "info",
			LogPrettyPrint:                    true,
			RegistrationControllerThreads:     1,
			ApplicationControllerThreads:      2,
			EnvironmentControllerThreads:      3,
			DeploymentControllerThreads:       4,
			JobControllerThreads:              5,
			AlertControllerThreads:            6,
			KubeClientRateLimitBurst:          100,
			KubeClientRateLimitQPS:            50.5,
			ReadinessProbeInitialDelaySeconds: 5,
			ReadinessProbePeriodSeconds:       10,

			DeploymentHistoryLimit: 10,

			DefaultRollingUpdateMaxUnavailable: "25%",
			DefaultRollingUpdateMaxSurge:       "35%",

			DefaultAppAdminGroups: []string{"default-app-admin-group1", "default-app-admin-group2"},

			AppNsLimitRange: config.LimitRangeConfig{
				DefaultMemory:        new(resource.MustParse("500M")),
				DefaultRequestMemory: new(resource.MustParse("450M")),
				DefaultRequestCPU:    new(resource.MustParse("100m")),
			},
			EnvNsLimitRange: config.LimitRangeConfig{
				DefaultMemory:        new(resource.MustParse("555M")),
				DefaultRequestMemory: new(resource.MustParse("444M")),
				DefaultRequestCPU:    new(resource.MustParse("111m")),
			},

			JobSchedulerImage: config.ContainerImage{
				Repository: "ghcr.io/equinor/radix-job-scheduler",
				Tag:        "v1.2.3",
			},
			JobSchedulerAuxImage: config.ContainerImage{
				Repository: "docker.io/bash",
				Tag:        "latest",
			},
			PodSecurityStandard: config.PodSecurityStandardConfig{
				AppNamespace: config.PodSecurityStandardPolicyConfig{
					Enforce: config.PodSecurityStandardModeConfig{
						Level:   "app-enforce-level",
						Version: "app-enforce-version",
					},
					Audit: config.PodSecurityStandardModeConfig{
						Level:   "app-audit-level",
						Version: "app-audit-version",
					},
					Warn: config.PodSecurityStandardModeConfig{
						Level:   "app-warn-level",
						Version: "app-warn-version",
					},
				},
				EnvNamespace: config.PodSecurityStandardPolicyConfig{
					Enforce: config.PodSecurityStandardModeConfig{
						Level:   "env-enforce-level",
						Version: "env-enforce-version",
					},
					Audit: config.PodSecurityStandardModeConfig{
						Level:   "env-audit-level",
						Version: "env-audit-version",
					},
					Warn: config.PodSecurityStandardModeConfig{
						Level:   "env-warn-level",
						Version: "env-warn-version",
					},
				},
			},
			BatchSafeToRestartJobThreshold: 1234,
			AzureKeyVaultTenantID:          "any-tenant-id",
			KubernetesAPIPort:              443,
			Gateway: config.GatewayConfig{
				Name:        "gateway",
				Namespace:   "istio-system",
				SectionName: "https",
			},
			CertificateAutomation: config.CertificateAutomationConfig{
				GatewayClusterIssuer: "any-cluster-issuer",
				Duration:             8760 * time.Hour,
				RenewBefore:          720 * time.Hour,
			},
			OrphanedEnvironmentsRetentionPeriod: 720 * time.Hour,
			OrphanedEnvironmentsCleanupCron:     "0 0 * * *",
			PipelineJobsHistoryLimit:            5,
			PipelineJobsHistoryPeriodLimit:      720 * time.Hour,

			PipelineImage: config.ContainerImage{
				Repository: "ghcr.io/equinor/radix-pipeline",
				Tag:        "v1.0.0",
			},
			PipelineImagePullPolicy: corev1.PullAlways,
		},
		Webhook: config.WebhookConfig{
			LogLevel:                 "info",
			LogPrettyPrint:           false,
			Port:                     9443,
			MetricsPort:              9000,
			HealthPort:               9440,
			RequireGroups:            true,
			RequireConfigurationItem: true,
			ReservedDNSAppAliases: map[string]string{
				"canary":   "radix-canary-golang",
				"console":  "radix-web-console",
				"cost-api": "radix-cost-allocation-api",
				"www":      "radix-public-site",
			},
			ReservedDNSAliases:                 []string{"grafana", "prometheus", "app", "playground", "dev", "api", "webhook"},
			SecretName:                         "radix-webhook-certs",
			SecretNamespace:                    "default",
			DisableCertRotation:                false,
			DNSName:                            "radix-webhook.example.svc",
			CAName:                             "radix-webhook-ca",
			CAOrganization:                     "Radix Webhook CA",
			CertsDir:                           "/run/certs",
			ExtraDNSNames:                      []string{"helloworld.example.svc"},
			ValidatingWebhookConfigurationName: "radix-webhook-configuration",
		},
	}

	assert.Equal(t, expected, cfg)
}

func TestParse_EnvOverride(t *testing.T) {
	t.Setenv("RADIXCONFIG_OPERATOR_LOGLEVEL", "debug")

	cfg, err := config.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Operator.LogLevel)
}

func TestParse_EnvMacro(t *testing.T) {
	t.Setenv("TEST_KUBERNETES_API_PORT", "6443")

	configYaml := strings.ReplaceAll(configHappyYaml, "kubernetesAPIPort: 443", `kubernetesAPIPort: "$__env(TEST_KUBERNETES_API_PORT)"`)

	cfg, err := config.Parse(configYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, int32(6443), cfg.Operator.KubernetesAPIPort)
}

// Only slice fields are comma separated, a scalar keeps the value as it is.
func TestParse_EnvOverrideDoesNotSplitStrings(t *testing.T) {
	t.Setenv("RADIXCONFIG_OPERATOR_LOGLEVEL", "debug,info")

	cfg, err := config.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "debug,info", cfg.Operator.LogLevel)
}
func TestParse_RequiredFieldFromEnvOverride(t *testing.T) {
	t.Setenv("RADIXCONFIG_COMMON_CLUSTERNAME", "env-cluster")
	configYamlStr := strings.ReplaceAll(configHappyYaml, "  clusterName: test-cluster\n", "")

	cfg, err := config.Parse(configYamlStr)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

// A field without an env tag is overridden by the uppercased field path, with dots replaced by underscores.
func TestParse_EnvOverrideFromFieldPath(t *testing.T) {
	t.Setenv("RADIXCONFIG_COMMON_OAUTH2PROXY_PROXYIMAGE_REPOSITORY", "ghcr.io/equinor/oauth2-proxy")
	t.Setenv("RADIXCONFIG_COMMON_OAUTH2PROXY_PROXYIMAGE_TAG", "v1.2.3")

	cfg, err := config.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	expected := config.ContainerImage{Repository: "ghcr.io/equinor/oauth2-proxy", Tag: "v1.2.3"}
	assert.Equal(t, expected, cfg.Common.OAuth2Proxy.ProxyImage)
}

func TestParse_EnvTagTakesPrecedenceOverFieldPath(t *testing.T) {
	t.Setenv("RADIXCONFIG_COMMON_CLUSTERNAME", "env-cluster")

	cfg, err := config.Parse(configHappyYaml)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "env-cluster", cfg.Common.ClusterName)
}

func TestParse_MissingRequiredField(t *testing.T) {
	cfg, err := config.Parse(configMissingRequiredYaml)

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParse_DeploymentHistoryLimitValidation(t *testing.T) {
	tests := map[string]struct {
		mutateConfig  MutateConfigFunc
		expectedError string
	}{
		"below 3 should fail": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.DeploymentHistoryLimit = 2
			},
			expectedError: `failed to validate config: field "Operator.DeploymentHistoryLimit" did not pass validation expression`,
		},
		"equal to 3 should pass": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.DeploymentHistoryLimit = 3
			},
			expectedError: ``,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.mutateConfig)

			cfg, err := config.Parse(configYaml)

			if test.expectedError == "" {
				require.NoError(t, err)
				return
			} else {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}

}
func TestParse_OrphanedEnvironmentsValidation(t *testing.T) {
	tests := map[string]struct {
		mutateConfig  MutateConfigFunc
		expectedError string
	}{
		"below 5 minutes should fail": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.OrphanedEnvironmentsRetentionPeriod = 4 * time.Minute
			},
			expectedError: `failed to validate config: field "Operator.OrphanedEnvironmentsRetentionPeriod" did not pass validation expression`,
		},
		"equal to 5 minutes should pass": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.OrphanedEnvironmentsRetentionPeriod = 5 * time.Minute
			},
			expectedError: ``,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.mutateConfig)

			cfg, err := config.Parse(configYaml)

			if test.expectedError == "" {
				require.NoError(t, err)
				return
			} else {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}

}

func TestParse_RequiredStructMustNotBeZero(t *testing.T) {
	configYaml := mutateConfig(t, func(cfg *config.Config) {
		cfg.Operator.JobSchedulerImage = config.ContainerImage{}
	})

	cfg, err := config.Parse(configYaml)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.ErrorContains(t, err, `field "Operator.JobSchedulerImage" is required but not set`)
}

func TestParse_FieldValidator(t *testing.T) {
	tests := map[string]struct {
		mutateConfig  MutateConfigFunc
		expectedError string
	}{
		"repository is required": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.JobSchedulerImage.Repository = ""
			},
			expectedError: `field "Operator.JobSchedulerImage" validation failed: repository is required`,
		},
		"tag is required": {
			mutateConfig: func(cfg *config.Config) {
				cfg.Operator.JobSchedulerImage.Tag = ""
			},
			expectedError: `field "Operator.JobSchedulerImage" validation failed: tag is required`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.mutateConfig)

			cfg, err := config.Parse(configYaml)

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.ErrorContains(t, err, test.expectedError)
		})
	}
}

func TestParse_BuilderResourceLimits(t *testing.T) {
	tests := map[string]struct {
		modifyConfig MutateConfigFunc
		errorPath    string
	}{
		"equivalent CPU quantities are valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.PipelineRunner.Builder.Resources.Limits.CPU = new(resource.MustParse("1"))
				cfg.PipelineRunner.Builder.Resources.Requests.CPU = new(resource.MustParse("1000m"))
			},
		},
		"CPU limit below request is invalid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.PipelineRunner.Builder.Resources.Limits.CPU = new(resource.MustParse("100m"))
			},
			errorPath: "PipelineRunner.Builder.Resources",
		},
		"memory limit below request is invalid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.PipelineRunner.Builder.Resources.Limits.Memory = new(resource.MustParse("499M"))
			},
			errorPath: "PipelineRunner.Builder.Resources",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.modifyConfig)

			cfg, err := config.Parse(configYaml)
			if test.errorPath == "" {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				return
			}

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.ErrorContains(t, err, test.errorPath)
		})
	}
}

func TestEnvConfigMapReader(t *testing.T) {

	tests := map[string]struct {
		env       map[string]string
		configMap *corev1.ConfigMap
		expectErr bool
	}{
		"defaults are used when env vars are not set": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": configHappyYaml},
			},
		},
		"env vars select namespace, name and key": {
			env: map[string]string{
				"POD_NAMESPACE":            "radix-system",
				"RADIX_COMMON_CONFIG_NAME": "custom-config",
				"RADIX_COMMON_CONFIG_KEY":  "customKey",
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-config", Namespace: "radix-system"},
				Data:       map[string]string{"customKey": configHappyYaml},
			},
		},
		"configmap not found": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"},
				Data:       map[string]string{"configYaml": configHappyYaml},
			},
			expectErr: true,
		},
		"key not found in configmap": {
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "radix-common-config", Namespace: "default"},
				Data:       map[string]string{"someOtherKey": configHappyYaml},
			},
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			client := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(test.configMap).Build()

			reader, err := config.EnvConfigMapReader(t.Context(), client)
			if test.expectErr {
				require.Error(t, err)
				assert.Empty(t, reader)
				return
			}
			require.NoError(t, err)

			cfg, err := config.Parse(reader)
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "test-cluster", cfg.Common.ClusterName)
			assert.Equal(t, "info", cfg.Operator.LogLevel)
		})
	}
}

func TestPipelineJobConfigs(t *testing.T) {
	tests := map[string]struct {
		modifyConfig MutateConfigFunc
		errorPath    string
	}{
		"Always is valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.Operator.PipelineImagePullPolicy = corev1.PullAlways
			},
		},
		"Never is valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.Operator.PipelineImagePullPolicy = corev1.PullNever
			},
		},
		"IfNotPresent is valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.Operator.PipelineImagePullPolicy = corev1.PullIfNotPresent
			},
		},
		"blank is not valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.Operator.PipelineImagePullPolicy = ""
			},
			errorPath: "Operator.PipelineImagePullPolicy",
		},
		"x is not valid": {
			modifyConfig: func(cfg *config.Config) {
				cfg.Operator.PipelineImagePullPolicy = "x"
			},
			errorPath: "Operator.PipelineImagePullPolicy",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			configYaml := mutateConfig(t, test.modifyConfig)

			cfg, err := config.Parse(configYaml)
			if test.errorPath == "" {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				return
			}

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.ErrorContains(t, err, test.errorPath)
		})
	}
}
