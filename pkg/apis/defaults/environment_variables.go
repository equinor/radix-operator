package defaults

const (
	// DeploymentsHistoryLimitEnvironmentVariable Controls the number of RDs we can have in a environment
	DeploymentsHistoryLimitEnvironmentVariable = "RADIX_DEPLOYMENTS_PER_ENVIRONMENT_HISTORY_LIMIT"

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

	// RadixCommitHashEnvironmentVariable Contains the commit id of the build
	RadixCommitHashEnvironmentVariable = "RADIX_GIT_COMMIT_HASH"

	// RadixGitTagsEnvironmentVariable Contains a list of git tags which the RADIX_GIT_COMMIT_HASH points to
	RadixGitTagsEnvironmentVariable = "RADIX_GIT_TAGS"

	// RadixComponentEnvironmentVariable The environment variable holding the name of the scheduled job
	RadixScheduleJobNameEnvironmentVariable = "RADIX_JOB_NAME"

	// RadixRestartEnvironmentVariable Environment variable to indicate that a restart was triggered
	RadixRestartEnvironmentVariable = "RADIX_RESTART_TRIGGERED"

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

	// RadixGitRefEnvironmentVariable When the pipeline job should be built from branch or tag specified here
	RadixGitRefEnvironmentVariable = "GIT_REF"

	// RadixGitRefTypeEnvironmentVariable When the pipeline job should be built from branch, tag or any specified in GIT_REF: tag, branch or empty
	RadixGitRefTypeEnvironmentVariable = "GIT_REF_TYPE"

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

	// KubernetesApiPortEnvironmentVariable Port which the K8s API server listens to for HTTPS
	KubernetesApiPortEnvironmentVariable = "KUBERNETES_SERVICE_PORT"

	// LogLevel Log level: ERROR, WARN, INFO (default), DEBUG
	LogLevel = "LOG_LEVEL"

	// RadixGithubWorkspaceEnvironmentVariable Path to a cloned GitHub repository
	RadixGithubWorkspaceEnvironmentVariable = "RADIX_GITHUB_WORKSPACE"

	// RadixSafeToRestartBatchJobThresholdVariable Threshold in seconds for determining cluster-autoscaler safe-to-evict annotation on batch jobs
	RadixSafeToRestartBatchJobThresholdVariable = "RADIXOPERATOR_SAFE_TO_RESTART_BATCH_JOB_THRESHOLD"
)
