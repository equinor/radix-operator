// Package oauth builds the EasyAuth OAuth2 client ID, redirect URIs and OAuth2 configuration of public components
package oauth

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

// ClientIDPrefix identifies Dex clients owned by EasyAuth, since Dex clients have no labels or metadata
const ClientIDPrefix = "easyauth."

// DefaultProxyPrefix is the OAuth2 proxy prefix set on public components when config.oauth2.proxyPrefix is empty.
// The prefix is always set, instead of using the radix-operator default, since EasyAuth must know the callback path to
// register the redirect URIs in Dex.
const DefaultProxyPrefix = "/oauth2"

// ClientID returns the Dex client ID of an application, see easyauth/README.md
func ClientID(appID string) string {
	return ClientIDPrefix + appID
}

// ClientIDForApp returns the Dex client ID of an application, from the AppID of its RadixRegistration
func ClientIDForApp(ctx context.Context, c client.Reader, appName string) (string, error) {
	rr := &radixv1.RadixRegistration{}
	if err := c.Get(ctx, client.ObjectKey{Name: appName}, rr); err != nil {
		return "", fmt.Errorf("failed to get RadixRegistration %s: %w", appName, err)
	}
	if rr.Spec.AppID.IsZero() {
		return "", fmt.Errorf("RadixRegistration %s has no appID", appName)
	}
	return ClientID(rr.Spec.AppID.String()), nil
}

// RedirectURI returns the OAuth2 proxy callback URL for a public host. The path is built from proxyPrefix the same way
// as radix-operator builds OAUTH2_PROXY_REDIRECT_URL.
func RedirectURI(host, proxyPrefix string) string {
	return fmt.Sprintf("https://%s%s/callback", host, oauthutil.SanitizePathPrefix(proxyPrefix))
}

// NewTemplate returns the OAuth2 configuration for public components, without the client ID. It is a copy of
// oauth2 from the EasyAuth config, with the credentials and OIDC issuer URL that EasyAuth requires, and
// DefaultProxyPrefix if no proxy prefix is set.
func NewTemplate(oauth2 radixv1.OAuth2, issuerURL string) *radixv1.OAuth2 {
	template := oauth2.DeepCopy()
	template.ClientID = ""
	template.Credentials = radixv1.Secret
	if template.ProxyPrefix == "" {
		template.ProxyPrefix = DefaultProxyPrefix
	}
	if template.OIDC == nil {
		template.OIDC = &radixv1.OAuth2OIDC{}
	}
	template.OIDC.IssuerURL = issuerURL
	return template
}

// CloneWithClientID returns the EasyAuth OAuth2 configuration for public components of an application, from template (see
// NewTemplate) and the client ID of the application
func CloneWithClientID(template *radixv1.OAuth2, clientID string) *radixv1.OAuth2 {
	oauth2 := template.DeepCopy()
	oauth2.ClientID = clientID
	return oauth2
}
