package defaults

import "fmt"

const (
	// RadixGithubWebhookRoleName Name of the service account representing the webhook
	RadixGithubWebhookRoleName = "radix-github-webhook"

	// RadixAPIRoleName Name of the service account representing the Radix API
	RadixAPIRoleName = "radix-api"

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

	// MachineUserPostfix Role (service account) of machine user will be <appName--machine-user> in default namespace
	MachineUserPostfix = "machine-user"

	// PlatformUserRoleName Name of platform user cluster role
	PlatformUserRoleName = "radix-platform-user"
)

// GetMachineUserRoleName Gets the name of the machine user for an app
func GetMachineUserRoleName(appName string) string {
	return fmt.Sprintf("%s-%s", appName, MachineUserPostfix)
}
