package defaults

const (
	// AzureACRServicePrincipleSecretName name of the secret containing ACR authentication information, consumed by az cli
	AzureACRServicePrincipleSecretName = "radix-sp-acr-azure"

	// AzureACRServicePrincipleBuildahSecretName name of the secret containing ACR authentication information, consumed by buildah
	AzureACRServicePrincipleBuildahSecretName = "radix-sp-buildah-azure"
)
