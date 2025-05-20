package defaults

const (
	// OperatorDNSZoneEnvironmentVariable The DNS zone used fro creating ingress of the cluster
	OperatorDNSZoneEnvironmentVariable = "DNS_ZONE"

	// OperatorAppAliasBaseURLEnvironmentVariable The base url for any app alias of the cluster
	OperatorAppAliasBaseURLEnvironmentVariable = "APP_ALIAS_BASE_URL"

	// OperatorClusterTypeEnvironmentVariable The type of cluster dev|playground|prod
	OperatorClusterTypeEnvironmentVariable = "RADIXOPERATOR_CLUSTER_TYPE"

	// DeploymentsHistoryLimitEnvironmentVariable Controls the number of RDs we can have in a environment
	DeploymentsHistoryLimitEnvironmentVariable = "RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT"

	// PipelineJobsHistoryLimitEnvironmentVariable Controls the number of RJs should exist in an app namespace, per groups by environment and status
	PipelineJobsHistoryLimitEnvironmentVariable = "RADIX_PIPELINE_JOBS_HISTORY_LIMIT"

	// PipelineJobsHistoryPeriodLimitEnvironmentVariable Controls how long an RJ should exist in an app namespace, per groups by environment and status
	PipelineJobsHistoryPeriodLimitEnvironmentVariable = "RADIX_PIPELINE_JOBS_HISTORY_PERIOD_LIMIT"

	// ClusternameEnvironmentVariable The name of the cluster
	ClusternameEnvironmentVariable = "RADIX_CLUSTERNAME"

	// ContainerRegistryEnvironmentVariable The name of the container registry
	ContainerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"

	// AppContainerRegistryEnvironmentVariable The name of the app container registry
	AppContainerRegistryEnvironmentVariable = "RADIX_APP_CONTAINER_REGISTRY"

	// AzureSubscriptionIdEnvironmentVariable The Azure subscription ID
	AzureSubscriptionIdEnvironmentVariable = "AZURE_SUBSCRIPTION_ID"

	// EnvironmentnameEnvironmentVariable The name of the environment for the application
	EnvironmentnameEnvironmentVariable = "RADIX_ENVIRONMENT"

	// PublicEndpointEnvironmentVariable The environment variable holding the public endpoint of the component
	PublicEndpointEnvironmentVariable = "RADIX_PUBLIC_DOMAIN_NAME"

	// CanonicalEndpointEnvironmentVariable Variable to hold the cluster spcific ingress
	CanonicalEndpointEnvironmentVariable = "RADIX_CANONICAL_DOMAIN_NAME"

	// RadixAppEnvironmentVariable The environment variable holding the name of the app
	RadixAppEnvironmentVariable = "RADIX_APP"

	// RadixComponentEnvironmentVariable The environment variable holding the name of the component
	RadixComponentEnvironmentVariable = "RADIX_COMPONENT"

	// RadixPortsEnvironmentVariable The environment variable holding the available ports of the component
	RadixPortsEnvironmentVariable = "RADIX_PORTS"

	// RadixPortNamesEnvironmentVariable The environment variable holding the available port names of the component
	RadixPortNamesEnvironmentVariable = "RADIX_PORT_NAMES"

	// RadixDNSZoneEnvironmentVariable The environment variable on a radix app giving the dns zone. Will be equal to OperatorDNSZoneEnvironmentVariable
	RadixDNSZoneEnvironmentVariable = "RADIX_DNS_ZONE"

	// RadixZoneEnvironmentVariable The environment variable on a radix app giving the radix zone.
	RadixZoneEnvironmentVariable = "RADIX_ZONE"

	// RadixClusterTypeEnvironmentVariable The type of cluster dev|playground|prod. Will be equal to OperatorClusterTypeEnvironmentVariable
	RadixClusterTypeEnvironmentVariable = "RADIX_CLUSTER_TYPE"

	// RadixCommitHashEnvironmentVariable Contains the commit id of the build
	RadixCommitHashEnvironmentVariable = "RADIX_GIT_COMMIT_HASH"

	// RadixGitTagsEnvironmentVariable Contains a list of git tags which the RADIX_GIT_COMMIT_HASH points to
	RadixGitTagsEnvironmentVariable = "RADIX_GIT_TAGS"

	// RadixComponentEnvironmentVariable The environment variable holding the name of the scheduled job
	RadixScheduleJobNameEnvironmentVariable = "RADIX_JOB_NAME"

	// RadixRestartEnvironmentVariable Environment variable to indicate that a restart was triggered
	RadixRestartEnvironmentVariable = "RADIX_RESTART_TRIGGERED"

	// RadixImageBuilderEnvironmentVariable Points to the image builder
	RadixImageBuilderEnvironmentVariable = "RADIX_IMAGE_BUILDER"

	// OperatorRadixJobSchedulerEnvironmentVariable Points to the image used to deploy job scheduler REST API for RD jobs
	OperatorRadixJobSchedulerEnvironmentVariable = "RADIXOPERATOR_JOB_SCHEDULER"

	// RadixDeploymentEnvironmentVariable Name of Radix Deployment
	RadixDeploymentEnvironmentVariable = "RADIX_DEPLOYMENT"

	// RadixComponentsToDeployVariable The optional list of components, which need to be deployed instead of all components
	RadixComponentsToDeployVariable = "RADIX_COMPONENTS_TO_DEPLOY"

	// RadixPromoteDeploymentEnvironmentVariable Name of Radix Deployment for pipeline
	RadixPromoteDeploymentEnvironmentVariable = "DEPLOYMENT_NAME"

	// RadixPromoteFromEnvironmentEnvironmentVariable Name of Radix environment for pipeline promote from
	RadixPromoteFromEnvironmentEnvironmentVariable = "FROM_ENVIRONMENT"

	// RadixPipelineJobToEnvironmentEnvironmentVariable Name of Radix environment for pipeline build-deploy or promote to
	RadixPipelineJobToEnvironmentEnvironmentVariable = "TO_ENVIRONMENT"

	// RadixPipelineJobTriggeredFromWebhookEnvironmentVariable Indicates that the pipeline job was triggered from a webhook
	RadixPipelineJobTriggeredFromWebhookEnvironmentVariable = "TRIGGERED_FROM_WEBHOOK"

	// RadixPromoteSourceDeploymentCommitHashEnvironmentVariable Git commit hash of source deployment in promote jobs
	RadixPromoteSourceDeploymentCommitHashEnvironmentVariable = "SOURCE_DEPLOYMENT_GIT_COMMIT_HASH"

	// RadixPromoteSourceDeploymentBranchEnvironmentVariable Git branch of source deployment in promote jobs
	RadixPromoteSourceDeploymentBranchEnvironmentVariable = "SOURCE_DEPLOYMENT_GIT_BRANCH"

	// RadixImageTagNameEnvironmentVariable Image tag name for Radix application components
	RadixImageTagNameEnvironmentVariable = "IMAGE_TAG_NAME"

	// RadixActiveClusterEgressIpsEnvironmentVariable IPs assigned to the cluster
	RadixActiveClusterEgressIpsEnvironmentVariable = "RADIX_ACTIVE_CLUSTER_EGRESS_IPS"

	// RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable Default OIDC issuer URL for OAuth Proxy
	RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable = "RADIX_OAUTH_PROXY_DEFAULT_OIDC_ISSUER_URL"

	// RadixOAuthProxyImageEnvironmentVariable specifies the name and tag of the OAuth Proxy image
	RadixOAuthProxyImageEnvironmentVariable = "RADIX_OAUTH_PROXY_IMAGE"

	// RadixConfigFileEnvironmentVariable Path to a radixconfig.yaml
	// to be loaded from Radix application config branch
	RadixConfigFileEnvironmentVariable = "RADIX_FILE_NAME"

	// RadixImageTagEnvironmentVariable Image tag for the built component
	RadixImageTagEnvironmentVariable = "IMAGE_TAG"

	// RadixPushImageEnvironmentVariable Push an image for the built component to an ACR
	RadixPushImageEnvironmentVariable = "PUSH_IMAGE"

	// RadixOverrideUseBuildCacheEnvironmentVariable override default or configured build cache option
	RadixOverrideUseBuildCacheEnvironmentVariable = "OVERRIDE_USE_BUILD_CACHE"

	// RadixRefreshBuildCacheEnvironmentVariable forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	RadixRefreshBuildCacheEnvironmentVariable = "REFRESH_BUILD_CACHE"

	// RadixPipelineJobEnvironmentVariable Radix pipeline job name
	RadixPipelineJobEnvironmentVariable = "JOB_NAME"

	// RadixBranchEnvironmentVariable Branch of the Radix application to process in a pipeline
	RadixBranchEnvironmentVariable = "BRANCH"

	// RadixGitRefsTypeEnvironmentVariable A target of the git event when the pipeline job is triggered by a GitHub event
	RadixGitRefsTypeEnvironmentVariable = "GIT_REFS_TYPE"

	// RadixConfigBranchEnvironmentVariable Branch of the Radix application config
	RadixConfigBranchEnvironmentVariable = "RADIX_CONFIG_BRANCH"

	// RadixCommitIdEnvironmentVariable Commit ID of the Radix application to process in a pipeline
	RadixCommitIdEnvironmentVariable = "COMMIT_ID"

	// RadixPipelineTypeEnvironmentVariable Pipeline type
	RadixPipelineTypeEnvironmentVariable = "PIPELINE_TYPE"

	// RadixPipelineTargetEnvironmentsVariable Pipeline target environments
	RadixPipelineTargetEnvironmentsVariable = "TARGET_ENVIRONMENTS"

	// RadixPipelineActionEnvironmentVariable Pipeline action: prepare, run
	RadixPipelineActionEnvironmentVariable = "RADIX_PIPELINE_ACTION"

	RadixPipelineApplyConfigDeployExternalDNSFlag = "APPLY_CONFIG_DEPLOY_EXTERNALDNS"

	// OperatorTenantIdEnvironmentVariable Tenant-id of the subscription
	OperatorTenantIdEnvironmentVariable = "RADIXOPERATOR_TENANT_ID"

	// KubernetesApiPortEnvironmentVariable Port which the K8s API server listens to for HTTPS
	KubernetesApiPortEnvironmentVariable = "KUBERNETES_SERVICE_PORT"

	// LogLevel Log level: ERROR, WARN, INFO (default), DEBUG
	LogLevel = "LOG_LEVEL"

	// PodSecurityStandardAppNamespaceEnforceLevelEnvironmentVariable Pod Security Standard enforce level for app namespaces
	PodSecurityStandardAppNamespaceEnforceLevelEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_APP_NAMESPACE_ENFORCE_LEVEL"

	// PodSecurityStandardEnforceLevelEnvironmentVariable Pod Security Standard enforce level for environment namespaces
	PodSecurityStandardEnforceLevelEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_ENFORCE_LEVEL"

	// PodSecurityStandardEnforceVersionEnvironmentVariable Pod Security Standard enforce version for app and environment namespaces
	PodSecurityStandardEnforceVersionEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_ENFORCE_VERSION"

	// PodSecurityStandardAuditLevelEnvironmentVariable Pod Security Standard audit level for app and environment namespaces
	PodSecurityStandardAuditLevelEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_AUDIT_LEVEL"

	// PodSecurityStandardAuditVersionEnvironmentVariable Pod Security Standard audit version for app and environment namespaces
	PodSecurityStandardAuditVersionEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_AUDIT_VERSION"

	// PodSecurityStandardWarnLevelEnvironmentVariable Pod Security Standard warn level for app and environment namespaces
	PodSecurityStandardWarnLevelEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_WARN_LEVEL"

	// PodSecurityStandardWarnVersionEnvironmentVariable Pod Security Standard warn version for app and environment namespaces
	PodSecurityStandardWarnVersionEnvironmentVariable = "RADIXOPERATOR_PODSECURITYSTANDARD_WARN_VERSION"

	// SeccompProfileFileNameEnvironmentVariable Filename of the seccomp profile injected by daemonset, relative to the /var/lib/kubelet/seccomp directory
	SeccompProfileFileNameEnvironmentVariable = "SECCOMP_PROFILE_FILENAME"

	// Deprecated: Radix no longer uses the buildah image directly. Use RadixBuildKitImageBuilderEnvironmentVariable
	// RadixBuildahImageBuilderEnvironmentVariable The container image used for running the buildah engine
	RadixBuildahImageBuilderEnvironmentVariable = "RADIX_BUILDAH_IMAGE_BUILDER"

	// RadixBuildKitImageBuilderEnvironmentVariable Repository and tag for the buildkit image builder
	RadixBuildKitImageBuilderEnvironmentVariable = "RADIX_BUILDKIT_IMAGE_BUILDER"

	// RadixReservedAppDNSAliasesEnvironmentVariable The list of DNS aliases, reserved for Radix platform Radix application
	RadixReservedAppDNSAliasesEnvironmentVariable = "RADIX_RESERVED_APP_DNS_ALIASES"

	// RadixReservedDNSAliasesEnvironmentVariable The list of DNS aliases, reserved for Radix platform services
	RadixReservedDNSAliasesEnvironmentVariable = "RADIX_RESERVED_DNS_ALIASES"

	// RadixCertificateAutomationClusterIssuerVariable Name of cluster isser to use for certificate automation
	RadixCertificateAutomationClusterIssuerVariable = "RADIXOPERATOR_CERTIFICATE_AUTOMATION_CLUSTER_ISSUER"

	// RadixCertificateAutomationDurationVariable Defines duration for certificates issued by cluster issuer
	RadixCertificateAutomationDurationVariable = "RADIXOPERATOR_CERTIFICATE_AUTOMATION_DURATION"

	// RadixCertificateAutomationRenewBeforeVariable Defines renew_before for certificates issued by cluster issuer
	RadixCertificateAutomationRenewBeforeVariable = "RADIXOPERATOR_CERTIFICATE_AUTOMATION_RENEW_BEFORE"

	// RadixPipelineImageTagEnvironmentVariable Radix pipeline image tag
	RadixPipelineImageTagEnvironmentVariable = "RADIXOPERATOR_PIPELINE_IMAGE_TAG"

	// RadixGitCloneNsLookupImageEnvironmentVariable The container image containing nslookup, used in pipeline git clone init containers
	RadixGitCloneNsLookupImageEnvironmentVariable = "RADIX_PIPELINE_GIT_CLONE_NSLOOKUP_IMAGE"

	// RadixGitCloneGitImageEnvironmentVariable The container image containing git, used in pipeline git clone init containers
	RadixGitCloneGitImageEnvironmentVariable = "RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE"

	// RadixGitCloneBashImageEnvironmentVariable The container image containing bash, used in pipeline git clone init containers
	RadixGitCloneBashImageEnvironmentVariable = "RADIX_PIPELINE_GIT_CLONE_BASH_IMAGE"

	// RadixExternalRegistryDefaultAuthEnvironmentVariable Name of the secret containing default credentials for external container registries.
	// Used when pulling images for components and jobs and for pulling images in Dockerfiles when building with buildah.
	RadixExternalRegistryDefaultAuthEnvironmentVariable = "RADIX_EXTERNAL_REGISTRY_DEFAULT_AUTH_SECRET"

	// RadixOrphanedEnvironmentsRetentionPeriodVariable The duration for which orphaned environments are retained
	RadixOrphanedEnvironmentsRetentionPeriodVariable = "RADIXOPERATOR_ORPHANED_ENVIRONMENTS_RETENTION_PERIOD"

	// RadixOrphanedEnvironmentsCleanupCronVariable The cron expression for cleaning up orphaned environments
	RadixOrphanedEnvironmentsCleanupCronVariable = "RADIXOPERATOR_ORPHANED_ENVIRONMENTS_CLEANUP_CRON"

	// RadixGithubWorkspaceEnvironmentVariable Path to a cloned GitHub repository
	RadixGithubWorkspaceEnvironmentVariable = "RADIX_GITHUB_WORKSPACE"
)
