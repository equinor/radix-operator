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

	// JobsHistoryLimitEnvironmentVariable Controls the number of RJs we can have in a app namespace
	JobsHistoryLimitEnvironmentVariable = "RADIX_JOBS_PER_APP_HISTORY_LIMIT"

	// ClusternameEnvironmentVariable The name of the cluster
	ClusternameEnvironmentVariable = "RADIX_CLUSTERNAME"

	// ContainerRegistryEnvironmentVariable The name of the container registry
	ContainerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"

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

	// RadixClusterTypeEnvironmentVariable The type of cluster dev|playground|prod. Will be equal to OperatorClusterTypeEnvironmentVariable
	RadixClusterTypeEnvironmentVariable = "RADIX_CLUSTER_TYPE"

	// ActiveClusternameEnvironmentVariable The name of the active cluster. If ActiveClusternameEnvironmentVariable == ClusternameEnvironmentVariable, this is the active cluster
	ActiveClusternameEnvironmentVariable = "RADIX_ACTIVE_CLUSTERNAME"

	// RadixCommitHashEnvironmentVariable Contains the commit id of the build
	RadixCommitHashEnvironmentVariable = "RADIX_GIT_COMMIT_HASH"

	// RadixRestartEnvironmentVariable Environment variable to indicate that a restart was triggered
	RadixRestartEnvironmentVariable = "RADIX_RESTART_TRIGGERED"

	// RadixImageBuilderEnvironmentVariable Points to the image builder
	RadixImageBuilderEnvironmentVariable = "RADIX_IMAGE_BUILDER"

	// RadixImageScannerEnvironmentVariable Points to the image scanner
	RadixImageScannerEnvironmentVariable = "RADIX_IMAGE_SCANNER"

	// OperatorRadixJobSchedulerEnvironmentVariable Points to the image used to deploy job scheduler REST API for RD jobs
	OperatorRadixJobSchedulerEnvironmentVariable = "RADIXOPERATOR_JOB_SCHEDULER"

	// RadixDeploymentEnvironmentVariable Name of Radix Deployment
	RadixDeploymentEnvironmentVariable = "RADIX_DEPLOYMENT"

	// RadixPromoteDeploymentEnvironmentVariable Name of Radix Deployment for pipeline
	RadixPromoteDeploymentEnvironmentVariable = "DEPLOYMENT_NAME"

	// RadixPromoteFromEnvironmentEnvironmentVariable Name of Radix environment for pipeline promote from
	RadixPromoteFromEnvironmentEnvironmentVariable = "FROM_ENVIRONMENT"

	// RadixPromoteToEnvironmentEnvironmentVariable Name of Radix environment for pipeline promote to
	RadixPromoteToEnvironmentEnvironmentVariable = "TO_ENVIRONMENT"

	// RadixDeploymentForceNonRootContainers Controls the non-root configuration for component containers
	// true: all component containers are force to run as non-root
	// false: non-root for a component container is controlled by runAsNonRoot from radixconfig
	RadixDeploymentForceNonRootContainers = "RADIX_DEPLOYMENTS_FORCE_NON_ROOT_CONTAINER"

	// RadixActiveClusterEgressIpsEnvironmentVariable IPs assigned to the cluster
	RadixActiveClusterEgressIpsEnvironmentVariable = "RADIX_ACTIVE_CLUSTER_EGRESS_IPS"

	// RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable Default OIDC issuer URL for OAuth Proxy
	RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable = "RADIX_OAUTH_PROXY_DEFAULT_OIDC_ISSUER_URL"

	// RadixOAuthProxyImageEnvironmentVariable specifies the name and tag of the OAuth Proxy image
	RadixOAuthProxyImageEnvironmentVariable = "RADIX_OAUTH_PROXY_IMAGE"

	// RadixTektonPipelineImageEnvironmentVariable Points to the utility image for preparing radixconfig copying
	///config/file to/map and preparing Tekton resources
	RadixTektonPipelineImageEnvironmentVariable = "RADIX_TEKTON_IMAGE"

	// RadixConfigFileEnvironmentVariable Path to a radixconfig.yaml
	// to be loaded from Radix application config branch
	RadixConfigFileEnvironmentVariable = "RADIX_FILE_NAME"

	// RadixImageTagEnvironmentVariable Image tag for the built component
	RadixImageTagEnvironmentVariable = "IMAGE_TAG"

	// RadixPushImageEnvironmentVariable Push an image for the built component to an ACR
	RadixPushImageEnvironmentVariable = "PUSH_IMAGE"

	// RadixUseCacheEnvironmentVariable Use cache for the built component
	RadixUseCacheEnvironmentVariable = "USE_CACHE"

	// RadixPipelineJobEnvironmentVariable Radix pipeline job name
	RadixPipelineJobEnvironmentVariable = "JOB_NAME"

	// RadixConfigConfigMapEnvironmentVariable Name of a ConfigMap with loaded radixconfig.yaml
	RadixConfigConfigMapEnvironmentVariable = "RADIX_CONFIG_CONFIGMAP"

	// RadixBranchEnvironmentVariable Branch of the Radix application to process in a pipeline
	RadixBranchEnvironmentVariable = "BRANCH"

	// RadixCommitIdEnvironmentVariable Commit ID of the Radix application to process in a pipeline
	RadixCommitIdEnvironmentVariable = "COMMIT_ID"

	// RadixPipelineTypeEnvironmentVariable Pipeline type
	RadixPipelineTypeEnvironmentVariable = "PIPELINE_TYPE"

	// RadixPipelineRunEnvironmentVariable Tekton Pipeline label for a Radix pipeline run
	RadixPipelineRunEnvironmentVariable = "RADIX_PIPELINE_RUN"

	// RadixTektonActionEnvironmentVariable Tekton action: prepare, run
	RadixTektonActionEnvironmentVariable = "RADIX_TEKTON_ACTION"

	// OperatorTenantIdEnvironmentVariable Tenant-id of the subscription
	OperatorTenantIdEnvironmentVariable = "RADIXOPERATOR_TENANT_ID"

	// KubernetesApiPortEnvironmentVariable Port which the K8s API server listens to for HTTPS
	KubernetesApiPortEnvironmentVariable = "KUBERNETES_SERVICE_PORT"

	// LogLevel Log level: ERROR, INFO (default), DEBUG
	LogLevel = "LOG_LEVEL"
)
