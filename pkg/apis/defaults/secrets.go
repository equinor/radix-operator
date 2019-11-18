package defaults

const (
	// BuildSecretPrefix All build secrets will be mounted with this prefix
	BuildSecretPrefix = "BUILD_SECRET_"

	// BuildSecretsName Name of the secret in the app namespace holding all build secrets
	BuildSecretsName = "build-secrets"

	// BuildSecretDefaultData When the build secrets hold radix_undefined, it means they have not been set yet
	BuildSecretDefaultData = "radix_undefined"
)
