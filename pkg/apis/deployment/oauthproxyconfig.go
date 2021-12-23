package deployment

import (
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/imdario/mergo"
)

func oauthConfigWithDefaults(config *v1.OAuth2) *v1.OAuth2 {
	target := &v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: utils.BoolPtr(false),
		SetAuthorizationHeader: utils.BoolPtr(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       "cookie",
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               os.Getenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable),
			InsecureSkipVerifyNonce: utils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:    "_oauth2_proxy",
			Expire:  "168h0m0s",
			Refresh: "60m0s",
		},
	}

	mergo.Merge(target, config, mergo.WithOverride, mergo.WithTransformers(transformer))
	return target
}
