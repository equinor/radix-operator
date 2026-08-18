package defaults

import (
	"dario.cat/mergo"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	mergoutils "github.com/equinor/radix-operator/pkg/apis/utils/mergo"
)

const (
	OAuthProxyPortName              = "http"
	OAuthProxyPortNumber      int32 = 4180
	OAuthProxyProviderOIDC          = "oidc"
	OAuthProxyProviderEntraId       = "entra-id"
)

// OAuth2Config is implemented by any value that has as MergeWith method
// The MergeWith method takes an OAuth2 object as input and merges it with an existing OAuth2 object
// The result of the merge is returned to the caller. The source object must not be modified
type OAuth2Config interface {
	MergeWith(source *v1.OAuth2) (*v1.OAuth2, error)
}

// OAuth2ConfigOptions defines configuration function for NewOAuth2Config
type OAuth2ConfigOptions func(cfg *oauth2Config)

// WithOIDCIssuerURL configures the OIDC.IssuerURL
func WithOIDCIssuerURL(url string) OAuth2ConfigOptions {
	return func(config *oauth2Config) {
		if config.OAuth2.OIDC == nil {
			config.OAuth2.OIDC = &v1.OAuth2OIDC{}
		}
		config.OAuth2.OIDC.IssuerURL = url
	}
}

// WithOIDCIssuerURL sets the default OAuth2 values
func WithOAuth2Defaults() OAuth2ConfigOptions {
	return func(cfg *oauth2Config) {
		cfg.OAuth2 = oauth2Default()
	}
}

// NewOAuth2Config returns a new object that implements OAuth2Config
func NewOAuth2Config(options ...OAuth2ConfigOptions) OAuth2Config {
	config := oauth2Config{OAuth2: v1.OAuth2{}}
	for _, option := range options {
		option(&config)
	}
	return &config
}

type oauth2Config struct {
	v1.OAuth2
}

func (cfg *oauth2Config) MergeWith(source *v1.OAuth2) (*v1.OAuth2, error) {
	target := cfg.OAuth2.DeepCopy()
	if err := mergo.Merge(target, source, mergo.WithOverride, mergo.WithTransformers(mergoutils.BoolPtrTransformer{})); err != nil {
		return nil, err
	}
	return target, nil
}

func oauth2Default() v1.OAuth2 {
	return v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: new(false),
		SetAuthorizationHeader: new(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       v1.SessionStoreCookie,
		OIDC: &v1.OAuth2OIDC{
			InsecureSkipVerifyNonce: new(false),
			SkipDiscovery:           new(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:     "_oauth2_proxy",
			Expire:   "168h0m0s",
			Refresh:  "60m0s",
			SameSite: v1.SameSiteLax,
		},
	}
}
