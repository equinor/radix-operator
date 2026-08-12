package defaults

const (
	// AzureACRServicePrincipleBuildahSecretName name of the secret containing ACR authentication information, consumed by buildah
	AzureACRServicePrincipleBuildahSecretName = "radix-sp-buildah-azure"

	// AzureACRTokenPasswordAppRegistrySecretName name of the secret containing Cache ACR authentication information, consumed by buildah
	AzureACRTokenPasswordAppRegistrySecretName = "radix-app-registry"

	GitPrivateKeySecretName = "git-ssh-keys"
	GitPrivateKeySecretKey  = "id_rsa"

	// WebhookSharedSecretName is the name of the secret holding the shared secret used to validate GitHub webhook payloads
	WebhookSharedSecretName = "radix-webhook-shared-secret"
	// WebhookSharedSecretKey is the data key in the WebhookSharedSecret secret holding the shared secret value
	WebhookSharedSecretKey = "sharedSecret"
)
