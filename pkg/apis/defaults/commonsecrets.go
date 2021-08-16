package defaults

const (
	// SnykServiceAccountSecretName name of the secret containing SNYK access token used in pipeline scan steps
	SnykServiceAccountSecretName = "radix-snyk-service-account"
	// AzureACRServicePrincipleSecretName name of the secret containing ACR authentication information
	AzureACRServicePrincipleSecretName = "radix-sp-acr-azure"
)
