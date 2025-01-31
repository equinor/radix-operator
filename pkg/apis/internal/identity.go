package internal

import "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// GetIdentityClientId Get the client id of the identity
func GetIdentityClientId(identity *v1.Identity) string {
	if identity != nil && identity.Azure != nil && len(identity.Azure.ClientId) > 0 {
		return identity.Azure.ClientId
	}
	return ""
}
