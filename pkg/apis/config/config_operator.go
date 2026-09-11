package config

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

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

	DefaultAppAdminGroups []string `json:"defaultAppAdminGroups"`

	ReadinessProbeInitialDelaySeconds int32 `json:"readinessProbeInitialDelaySeconds" required:"true"`
	ReadinessProbePeriodSeconds       int32 `json:"readinessProbePeriodSeconds" required:"true"`

	DefaultRollingUpdateMaxUnavailable string `json:"defaultRollingUpdateMaxUnavailable" required:"true"`
	DefaultRollingUpdateMaxSurge       string `json:"defaultRollingUpdateMaxSurge" required:"true"`

	AppNsLimitRange LimitRangeConfig `json:"appNsLimitRange" required:"true"`
	EnvNsLimitRange LimitRangeConfig `json:"envNsLimitRange" required:"true"`

	JobSchedulerImage   ContainerImage            `json:"jobSchedulerImage" required:"true"`
	PodSecurityStandard PodSecurityStandardConfig `json:"podSecurityStandard"`

	BatchSafeToRestartJobThreshold int64 `json:"batchSafeToRestartJobThreshold" required:"true"`

	AzureKeyVaultTenantID string `json:"azureKeyVaultTenantID" required:"true"`

	JobSchedulerAuxImage ContainerImage `json:"jobSchedulerAuxImage" required:"true"`

	KubernetesAPIPort                   int32                       `json:"kubernetesAPIPort" required:"true"`
	DeploymentHistoryLimit              int                         `json:"deploymentHistoryLimit" required:"true" validate:"self >= 3"`
	Gateway                             GatewayConfig               `json:"gateway" required:"true"`
	CertificateAutomation               CertificateAutomationConfig `json:"certificateAutomation" required:"true"`
	OrphanedEnvironmentsRetentionPeriod time.Duration               `json:"orphanedEnvironmentsRetentionPeriod" required:"true" validate:"compareDuration(self, '5m') >= 0"`
	OrphanedEnvironmentsCleanupCron     string                      `json:"orphanedEnvironmentsCleanupCron" required:"true"`

	PipelineJobsHistoryLimit       int               `json:"pipelineJobsHistoryLimit" required:"true" validate:"self >= 3"`
	PipelineJobsHistoryPeriodLimit time.Duration     `json:"pipelineJobsHistoryPeriodLimit" required:"true" validate:"compareDuration(self, '24h') >= 0"`
	PipelineImage                  ContainerImage    `json:"pipelineImage" required:"true"`
	PipelineImagePullPolicy        corev1.PullPolicy `json:"pipelineImagePullPolicy" required:"true" validate:"self in ['Always','IfNotPresent','Never']"`
}
