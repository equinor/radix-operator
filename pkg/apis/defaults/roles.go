package defaults

const (
	// RadixGithubWebhookRoleName Name of the cluster role with RBAC for radix-github-webhook service account
	RadixGithubWebhookRoleName = "radix-github-webhook"

	// RadixGithubWebhookServiceAccountName Name of the service account representing the webhook
	RadixGithubWebhookServiceAccountName = "radix-github-webhook"

	// RadixAPIRoleName Name of the cluster role with RBAC for radix-api service account
	RadixAPIRoleName = "radix-api"

	// RadixAPIAccessValidationRoleName Name of the cluster role with RBAC for radix-api access validation
	RadixAPIAccessValidationRoleName = "radix-api-access-validation"

	// RadixAPIServiceAccountName Name of the service account representing the Radix API
	RadixAPIServiceAccountName = "radix-api"

	// AppAdminRoleName Name of role which grants access to manage the CI/CD of their applications
	AppAdminRoleName = "radix-app-admin"

	// AppReaderRoleName Name of role which grants read access to the CI/CD of their applications
	AppReaderRoleName = "radix-app-reader"

	// AppAdminEnvironmentRoleName Name of role which grants access to manage their running Radix applications
	AppAdminEnvironmentRoleName = "radix-app-admin-envs"

	// AppReaderEnvironmentsRoleName Name of role which grants read access to their running Radix applications
	AppReaderEnvironmentsRoleName = "radix-app-reader-envs"

	// PipelineServiceAccountName Service account name for the pipeline
	PipelineServiceAccountName = "radix-pipeline"

	// PipelineAppRoleName Role to update the radix config from repo and execute the outer pipeline
	PipelineAppRoleName = "radix-pipeline-app"

	// PipelineEnvRoleName Give radix-pipeline service account inside app namespace access to make deployments through radix-pipeline-runner clusterrole
	PipelineEnvRoleName = "radix-pipeline-env"

	// RadixTektonServiceAccountName Service account name for radix-tekton jobs
	RadixTektonServiceAccountName = "radix-tekton"

	// RadixTektonAppRoleName Role of user to apply radixconfig to configmap and process Tekton objects
	RadixTektonAppRoleName = "radix-tekton-app"

	// RadixTektonEnvRoleName Role of user to get app environment data for prepare pipeline job
	RadixTektonEnvRoleName = "radix-tekton-env"

	// PlatformUserRoleName Name of platform user cluster role
	PlatformUserRoleName = "radix-platform-user"

	// RadixJobSchedulerRoleName Name of the cluster role with RBAC for radix-job-scheduler service account
	RadixJobSchedulerRoleName = "radix-job-scheduler"

	// RadixJobSchedulerServiceName Name of the service account representing the Radix Job Scheduler
	RadixJobSchedulerServiceName = "radix-job-scheduler"
)
