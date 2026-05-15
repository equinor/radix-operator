package token

import (
	"context"
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	test_oidc_audience = "testaudience"
	test_jwt           = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyIsImtpZCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyJ9.eyJhdWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MjQ1YTJlYzEiLCJpc3MiOiJodHRwczovL3N0cy53aW5kb3dzLm5ldC8xMjM0NTY3OC03NTY1LTIzNDItMjM0Mi0xMjM0MDViNDU5YjAvIiwiaWF0IjoxNTc1MzU1NTA4LCJuYmYiOjE1NzUzNTU1MDgsImV4cCI6MTU3NTM1OTQwOCwiYWNyIjoiMSIsImFpbyI6IjQyYXNkYXMiLCJhbXIiOlsicHdkIl0sImFwcGlkIjoiMTIzNDU2NzgtMTIzNC0xMjM0LTEyMzQtMTIzNDc5MDM5YTkwIiwiYXBwaWRhY3IiOiIwIiwiZmFtaWx5X25hbWUiOiJKb2huIiwiZ2l2ZW5fbmFtZSI6IkRvZSIsImhhc2dyb3VwcyI6InRydWUiLCJpcGFkZHIiOiIxMC4xMC4xMC4xMCIsIm5hbWUiOiJKb2huIERvZSIsIm9pZCI6IjEyMzQ1Njc4LTEyMzQtMTIzNC0xMjM0LTEyMzRmYzhmYTBlYSIsIm9ucHJlbV9zaWQiOiJTLTEtNS0yMS0xMjM0NTY3ODktMTIzNDU2OTc4MC0xMjM0NTY3ODktMTIzNDU2NyIsInNjcCI6InVzZXJfaW1wZXJzb25hdGlvbiIsInN1YiI6IjBoa2JpbEo3MTIzNHpSU3h6eHZiSW1hc2RmZ3N4amI2YXNkZmVOR2FzZGYiLCJ0aWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MDViNDU5YjAiLCJ1bmlxdWVfbmFtZSI6Im5vdC1leGlzdGluZy1yYWRpeC1lbWFpbEBlcXVpbm9yLmNvbSIsInVwbiI6Im5vdC1leGlzdGluZy10ZXN0LXJhZGl4LWVtYWlsQGVxdWlub3IuY29tIiwidXRpIjoiQlMxMmFzR2R1RXlyZUVjRGN2aDJBRyIsInZlciI6IjEuMCJ9.EB5z7Mk34NkFPCP8MqaNMo4UeWgNyO4-qEmzOVPxfoBqbgA16Ar4xeONXODwjZn9iD-CwJccusW6GP0xZ_PJHBFpfaJO_tLaP1k0KhT-eaANt112TvDBt0yjHtJg6He6CEDqagREIsH3w1mSm40zWLKGZeRLdnGxnQyKsTmNJ1rFRdY3AyoEgf6-pnJweUt0LaFMKmIJ2HornStm2hjUstBaji_5cSS946zqp4tgrc-RzzDuaQXzqlVL2J22SR2S_Oux_3yw88KmlhEFFP9axNcbjZrzW3L9XWnPT6UzVIaVRaNRSWfqDATg-jeHg4Gm1bp8w0aIqLdDxc9CfFMjuQ"
)

func TestValidUser(t *testing.T) {
	issuer := createServer(t)
	v, err := NewValidator(issuer, test_oidc_audience)
	assert.NoError(t, err)
	assert.NotNil(t, v)

	principal, err := v.ValidateToken(context.Background(), createUser(t, issuer, test_oidc_audience, "user1"))
	require.NoError(t, err)
	assert.NotNil(t, principal)
	assert.Equal(t, "sub:user1", principal.Id())
	assert.Equal(t, "user1", principal.Name())
}

func TestInvalidToken(t *testing.T) {
	issuer := createServer(t)
	v, err := NewValidator(issuer, test_oidc_audience)
	assert.NoError(t, err)
	assert.NotNil(t, v)

	_, err = v.ValidateToken(context.Background(), createUser(t, issuer, "invalid audience", "user1"))
	assert.Error(t, err)
}
func TestValidUserMultipleIssuers(t *testing.T) {
	issuer1 := createServer(t)
	issuer2 := createServer(t)
	v1, err := NewValidator(issuer1, test_oidc_audience)
	assert.NoError(t, err)
	v2, err := NewValidator(issuer2, test_oidc_audience)
	assert.NoError(t, err)
	v := NewChainedValidator(v1, v2)

	principal, err := v.ValidateToken(context.Background(), createUser(t, issuer2, test_oidc_audience, "user1"))
	require.NoError(t, err)
	assert.NotNil(t, principal)
	assert.Equal(t, "sub:user1", principal.Id())
	assert.Equal(t, "user1", principal.Name())

	_, err = v.ValidateToken(context.Background(), createUser(t, issuer1, "invalid audience", "user1"))
	assert.Error(t, err)

	fakeIssuer, err := url.Parse("http://fakeissuer/")
	require.NoError(t, err)
	_, err = v.ValidateToken(context.Background(), createUser(t, *fakeIssuer, test_oidc_audience, "user1"))
	assert.Error(t, err)

	_, err = v.ValidateToken(context.Background(), test_jwt)
	assert.Error(t, err)
}

func createServer(t *testing.T) url.URL {
	m, err := mockoidc.Run()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = m.Shutdown()
	})

	issuer, err := url.Parse(m.Issuer())
	require.NoError(t, err)

	return *issuer
}

func createUser(t *testing.T, issuer url.URL, audience, subject string) string {
	user := mockoidc.DefaultUser()
	key, err := mockoidc.DefaultKeypair()
	require.NoError(t, err)

	claims, err := user.Claims([]string{"profile", "email"}, &mockoidc.IDTokenClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:   issuer.String(),
			Subject:  subject,
			Audience: []string{audience},
		},
	})
	require.NoError(t, err)

	signJWT, err := key.SignJWT(claims)
	require.NoError(t, err)
	return signJWT
}
