package deployment

import (
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/imdario/mergo"
)

type OAuth2ConfigFunc func(*v1.OAuth2) *v1.OAuth2

func (f OAuth2ConfigFunc) MergeWithDefaults(source *v1.OAuth2) *v1.OAuth2 {
	return f(source)
}

// OAuth2Config defines a method for providing default values for undefined properties in an OAuth2 config
type OAuth2Config interface {
	MergeWithDefaults(source *v1.OAuth2) *v1.OAuth2
}

func oauth2ConfigDefaults() *v1.OAuth2 {
	return &v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: utils.BoolPtr(false),
		SetAuthorizationHeader: utils.BoolPtr(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       v1.SessionStoreCookie,
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
}

func oauth2ConfigFuncImpl(source *v1.OAuth2) *v1.OAuth2 {
	target := oauth2ConfigDefaults()
	mergo.Merge(target, source, mergo.WithOverride, mergo.WithTransformers(authTransformer))
	return target
}
