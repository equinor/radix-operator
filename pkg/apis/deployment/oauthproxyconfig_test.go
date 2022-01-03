package deployment

import (
	"encoding/base64"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func TestOAuthProxyConfigDefaults(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 20)
	rand.Read(b)
	defaultIssuerUrl := base64.URLEncoding.EncodeToString(b)
	os.Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, defaultIssuerUrl)
	defer os.Unsetenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)

	expected := &v1.OAuth2{
		Scope:                  "openid profile email",
		SetXAuthRequestHeaders: utils.BoolPtr(false),
		SetAuthorizationHeader: utils.BoolPtr(false),
		ProxyPrefix:            "/oauth2",
		SessionStoreType:       "cookie",
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               defaultIssuerUrl,
			InsecureSkipVerifyNonce: utils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:    "_oauth2_proxy",
			Expire:  "168h0m0s",
			Refresh: "60m0s",
		},
	}
	actual := oauthConfigWithDefaults(&v1.OAuth2{})
	assert.Equal(t, expected, actual)
}
