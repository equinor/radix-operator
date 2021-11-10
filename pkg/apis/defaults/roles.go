package defaults

import "fmt"

const (
	// RadixGithubWebhookRoleName Name of the cluster role with RBAC for radix-github-webhook service account
	RadixGithubWebhookRoleName = "radix-github-webhook"

	// RadixGithubWebhookServiceAccountName Name of the service account representing the webhook
	RadixGithubWebhookServiceAccountName = "radix-github-webhook"

	// RadixAPIRoleName Name of the cluster role with RBAC for radix-api service account
	RadixAPIRoleName = "radix-api"

	// RadixAPIServiceAccountName Name of the service account representing the Radix API
	RadixAPIServiceAccountName = "radix-api"

	// AppAdminRoleName Name of role which grants access to manage the CI/CD of their applications
	AppAdminRoleName = "radix-app-admin"

	// AppAdminEnvironmentRoleName Name of role which grants access to manage their running Radix applications
	AppAdminEnvironmentRoleName = "radix-app-admin-envs"

	// PipelineRoleName Role to update the radix config from repo and execute the outer pipeline
	PipelineRoleName = "radix-pipeline"

	// PipelineRunnerRoleName Give radix-pipeline service account inside app namespace access to creating namespaces and make deployments through radix-pipeline-runner clusterrole
	PipelineRunnerRoleName = "radix-pipeline-runner"

	// ConfigToMapRunnerRoleName Role (service account name) of user to apply radixconfig to configmap
	ConfigToMapRunnerRoleName = "radix-config-to-map-runner"

	// ScanImageRunnerRoleName Role (service account name) of user to store vulnerability scan results to configmap
	ScanImageRunnerRoleName = "radix-scan-image-runner"

	// MachineUserPostfix Role (service account) of machine user will be <appName--machine-user> in default namespace
	MachineUserPostfix = "machine-user"

	// PlatformUserRoleName Name of platform user cluster role
	PlatformUserRoleName = "radix-platform-user"

	// RadixJobSchedulerServerRoleName Name of the cluster role with RBAC for radix-jpob-scheduler service account
	RadixJobSchedulerServerRoleName = "radix-job-scheduler-server"

	// RadixJobSchedulerServerServiceName Name of the service account representing the Radix Job Scheduler
	RadixJobSchedulerServerServiceName = "radix-job-scheduler-server"
)

// GetMachineUserRoleName Gets the name of the machine user for an app
func GetMachineUserRoleName(appName string) string {
	return fmt.Sprintf("%s-%s", appName, MachineUserPostfix)
}
