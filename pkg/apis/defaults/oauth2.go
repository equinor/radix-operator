package defaults

import (
	commonUtils "github.com/equinor/radix-common/utils"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/imdario/mergo"
)

type OAuth2DefaultConfigApplier interface {
	ApplyTo(source *v1.OAuth2) (*v1.OAuth2, error)
}

type OAuth2DefaultConfigOptions func(cfg *OAuth2DefaultConfig)

func WithOIDCIssuerURL(url string) OAuth2DefaultConfigOptions {
	return func(config *OAuth2DefaultConfig) {
		if config.OAuth2.OIDC == nil {
			config.OAuth2.OIDC = &v1.OAuth2OIDC{}
		}
		config.OAuth2.OIDC.IssuerURL = url
	}
}

func NewOAuth2DefaultConfig(options ...OAuth2DefaultConfigOptions) OAuth2DefaultConfig {
	config := OAuth2DefaultConfig{OAuth2: oauth2Default()}
	for _, option := range options {
		option(&config)
	}
	return config
}

type OAuth2DefaultConfig struct {
	v1.OAuth2
}

func (cfg *OAuth2DefaultConfig) ApplyTo(source *v1.OAuth2) (*v1.OAuth2, error) {
	authTransformer := mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
	target := cfg.OAuth2.DeepCopy()
	if err := mergo.Merge(target, source, mergo.WithOverride, mergo.WithTransformers(authTransformer)); err != nil {
		return nil, err
	}
	return target, nil
}

func oauth2Default() v1.OAuth2 {
	return v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: commonUtils.BoolPtr(false),
		SetAuthorizationHeader: commonUtils.BoolPtr(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       v1.SessionStoreCookie,
		OIDC: &v1.OAuth2OIDC{
			InsecureSkipVerifyNonce: commonUtils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:     "_oauth2_proxy",
			Expire:   "168h0m0s",
			Refresh:  "60m0s",
			SameSite: v1.SameSiteLax,
		},
	}
}
