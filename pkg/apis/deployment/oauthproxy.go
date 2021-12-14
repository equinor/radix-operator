package deployment

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/imdario/mergo"
)

func (deploy *Deployment) createOrUpdateOAuthProxy(component v1.RadixCommonDeployComponent) error {
	isPublic := component.GetPublicPort() != "" || component.IsPublic()

	if auth := component.GetAuthentication(); auth != nil && auth.OAuth2 != nil && isPublic {
		return deploy.oauthProxyResourceManager.Install(component)
	} else {
		return deploy.oauthProxyResourceManager.Uninstall(component.GetName())
	}
}

// func WithDefaults(source *v1.OAuth2) (*v1.OAuth2, error) {
// 	// authBase := componentAuthentication.DeepCopy()
// 	// authEnv := environmentAuthentication.DeepCopy()
// 	// if err := mergo.Merge(authBase, authEnv, mergo.WithOverride, mergo.WithTransformers(transformer)); err != nil {
// 	// 	return nil, err
// 	// }
// 	return nil, nil
// }

func oauth2DefaultsWithSource(source *v1.OAuth2) *v1.OAuth2 {
	target := &v1.OAuth2{
		Scope:                  "openid profile email offline_acccess",
		SetXAuthRequestHeaders: utils.BoolPtr(false),
		SetAuthorizationHeader: utils.BoolPtr(false),
		EmailDomain:            "equinor.com",
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       "cookie",
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               "https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/v2.0",
			EmailClaim:              "email",
			GroupsClaim:             "groups",
			InsecureSkipVerifyNonce: utils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:    "_oauth2_proxy",
			Expire:  "168h0m0s",
			Refresh: "60m0s",
			Path:    "/",
		},
	}

	mergo.Merge(target, source, mergo.WithOverride, mergo.WithTransformers(transformer))
	return target
}
