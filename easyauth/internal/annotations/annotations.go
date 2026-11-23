// Package annotations defines the annotations used by EasyAuth
package annotations

const (
	// EasyAuth enables EasyAuth on a RadixApplication. The value must be empty.
	EasyAuth = "easyauth.radix.equinor.com"
	// AllowedIdentities is a comma separated list of groups allowed to access the public components of a
	// RadixApplication. Replaces oauth2.allowedGroups in the config, and is required when it is empty.
	AllowedIdentities = "easyauth.radix.equinor.com/allowed-identities"
	// RestartPending is set on the OAuth2 proxy Secret when the client secret has changed, until the OAuth2 proxy is restarted
	RestartPending = "easyauth.radix.equinor.com/restart-pending"
	// RestartedAt is set on the pod template of the OAuth2 proxy Deployment to restart it
	RestartedAt = "easyauth.radix.equinor.com/restartedAt"
)
