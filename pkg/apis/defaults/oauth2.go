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

func MergeOAuth2(original, override v1.OAuth2) (v1.OAuth2, error) {
	tmpTarget := original.DeepCopy()
	if err := mergo.Merge(tmpTarget, &override, mergo.WithOverride, mergo.WithTransformers(mergoutils.BoolPtrTransformer{})); err != nil {
		return v1.OAuth2{}, err
	}
	return *tmpTarget, nil
}
