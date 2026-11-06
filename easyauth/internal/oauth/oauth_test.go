package oauth

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
)

const (
	issuerURL = "https://simpleauth.example.com"
	appID     = "01JABCDEFGHJKMNPQRSTVWXYZ0"
)

var oauth2 = radixv1.OAuth2{
	Scope:                  "openid profile email groups offline_access",
	SetAuthorizationHeader: new(false),
	SetXAuthRequestHeaders: new(false),
	SetUserHeaders:         new(true),
	SessionStoreType:       radixv1.SessionStoreSystemManaged,
}

func TestClientID(t *testing.T) {
	assert.Equal(t, "easyauth."+appID, ClientID(appID))
}

func TestClientIDForApp(t *testing.T) {
	id := ulid.Make()
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(
		&radixv1.RadixRegistration{Name: "with-id", Spec: radixv1.RadixRegistrationSpec{AppID: radixv1.ULID{ULID: id}}},
		&radixv1.RadixRegistration{Name: "without-id"},
	).Build()

	got, err := ClientIDForApp(context.Background(), c, "with-id")
	require.NoError(t, err)
	assert.Equal(t, "easyauth."+id.String(), got)

	_, err = ClientIDForApp(context.Background(), c, "without-id")
	assert.Error(t, err)

	_, err = ClientIDForApp(context.Background(), c, "missing")
	assert.Error(t, err)
}

func TestRedirectURI(t *testing.T) {
	for prefix, expected := range map[string]string{
		"/oauth2":   "https://a.example.com/oauth2/callback",
		"oauth2":    "https://a.example.com/oauth2/callback",
		"/auth/":    "https://a.example.com/auth/callback",
		"/a/../b c": "https://a.example.com/b%20c/callback",
	} {
		assert.Equal(t, expected, RedirectURI("a.example.com", prefix), prefix)
	}
}

func TestNewTemplate(t *testing.T) {
	t.Run("sets credentials and issuer", func(t *testing.T) {
		template := NewTemplate(oauth2, issuerURL)
		expected := &radixv1.OAuth2{
			Credentials:            radixv1.Secret,
			Scope:                  "openid profile email groups offline_access",
			SetAuthorizationHeader: new(false),
			SetXAuthRequestHeaders: new(false),
			SetUserHeaders:         new(true),
			SessionStoreType:       radixv1.SessionStoreSystemManaged,
			ProxyPrefix:            DefaultProxyPrefix,
			OIDC:                   &radixv1.OAuth2OIDC{IssuerURL: issuerURL},
		}
		assert.Equal(t, expected, template)
	})

	t.Run("keeps other OIDC settings and does not modify the input", func(t *testing.T) {
		in := radixv1.OAuth2{
			ClientID:    "ignored",
			Credentials: radixv1.AzureWorkloadIdentity,
			OIDC:        &radixv1.OAuth2OIDC{IssuerURL: "https://other.example.com", SkipDiscovery: new(true)},
		}
		template := NewTemplate(in, issuerURL)

		assert.Empty(t, template.ClientID)
		assert.Equal(t, radixv1.Secret, template.Credentials)
		assert.Equal(t, &radixv1.OAuth2OIDC{IssuerURL: issuerURL, SkipDiscovery: new(true)}, template.OIDC)
		assert.Equal(t, "https://other.example.com", in.OIDC.IssuerURL)
	})

	t.Run("keeps a configured proxy prefix", func(t *testing.T) {
		template := NewTemplate(radixv1.OAuth2{ProxyPrefix: "/auth"}, issuerURL)
		assert.Equal(t, "/auth", template.ProxyPrefix)
	})
}

func TestForApp_ReturnsCopy(t *testing.T) {
	template := NewTemplate(oauth2, issuerURL)
	config := ForApp(template, ClientID(appID), nil)
	assert.Equal(t, ClientID(appID), config.ClientID)

	config.OIDC.IssuerURL = "changed"
	*config.SetUserHeaders = false

	assert.Empty(t, template.ClientID)
	assert.Equal(t, issuerURL, template.OIDC.IssuerURL)
	assert.True(t, *template.SetUserHeaders)
}

func TestForApp_AllowedIdentities(t *testing.T) {
	config := oauth2
	config.AllowedIdentities = []string{"default-a", "default-b"}
	template := NewTemplate(config, issuerURL)

	assert.Equal(t, []string{"default-a", "default-b"}, ForApp(template, ClientID(appID), nil).AllowedIdentities,
		"default groups are used without allowed identities")

	groups := []string{"group-a", "group-b"}
	result := ForApp(template, ClientID(appID), groups)
	assert.Equal(t, []string{"group-a", "group-b"}, result.AllowedIdentities, "allowed identities replace the default groups")

	result.AllowedIdentities[0] = "changed"
	ForApp(template, ClientID(appID), nil).AllowedIdentities[0] = "changed"
	assert.Equal(t, []string{"group-a", "group-b"}, groups, "allowed identities must not be changed")
	assert.Equal(t, []string{"default-a", "default-b"}, template.AllowedIdentities, "template must not be changed")
}

func TestSplitIdentities(t *testing.T) {
	assert.Nil(t, SplitIdentities(""))
	assert.Equal(t, []string{"group-a"}, SplitIdentities("group-a"))
	assert.Equal(t, []string{"group-a", "group-b"}, SplitIdentities("group-a,group-b"))
	assert.Equal(t, []string{"group-a", ""}, SplitIdentities("group-a,"))
}
