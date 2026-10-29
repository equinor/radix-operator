package defaulter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	annotation = "radix.equinor.com/easyauth"
	issuerURL  = "https://simpleauth.example.com"
)

func expectedOAuth2(appName string) *radixv1.OAuth2 {
	return &radixv1.OAuth2{
		ClientID:               appName + ".dev.radix.equinor.com",
		Credentials:            radixv1.Secret,
		Scope:                  "openid profile email offline_access",
		SetAuthorizationHeader: new(false),
		SetXAuthRequestHeaders: new(false),
		OIDC:                   &radixv1.OAuth2OIDC{IssuerURL: issuerURL},
	}
}

func TestDefault_OverridesOAuth2OnPublicComponents(t *testing.T) {
	ra := &radixv1.RadixApplication{
		Name:        "my-app",
		Annotations: map[string]string{annotation: ""},
		Spec: radixv1.RadixApplicationSpec{
			Components: []radixv1.RadixComponent{
				{Name: "public-port", PublicPort: "http"},
				{Name: "public-deprecated", Public: true},
				{
					Name:       "public-existing-oauth",
					PublicPort: "http",
					Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{
						ClientID:       "other",
						SkipAuthRoutes: []string{"^/healthz$"},
					}},
					EnvironmentConfig: []radixv1.RadixEnvironmentConfig{
						{Environment: "dev", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "env"}}},
						{Environment: "prod"},
					},
				},
				{Name: "private"},
			},
		},
	}

	d := &RadixApplicationDefaulter{Annotation: annotation, OAuth2IssuerURL: issuerURL}
	require.NoError(t, d.Default(context.Background(), ra))

	for _, name := range []string{"public-port", "public-deprecated", "public-existing-oauth"} {
		component := findComponent(t, ra, name)
		require.NotNil(t, component.Authentication, name)
		assert.Equal(t, expectedOAuth2("my-app"), component.Authentication.OAuth2, name)
	}

	envConfigs := findComponent(t, ra, "public-existing-oauth").EnvironmentConfig
	assert.Nil(t, envConfigs[0].Authentication.OAuth2, "environment OAuth2 config should be removed")
	assert.Nil(t, envConfigs[1].Authentication)

	assert.Nil(t, findComponent(t, ra, "private").Authentication, "private components should not be changed")
}

func TestDefault_WithoutAnnotationDoesNothing(t *testing.T) {
	ra := &radixv1.RadixApplication{
		Name: "my-app",
		Spec: radixv1.RadixApplicationSpec{
			Components: []radixv1.RadixComponent{{Name: "web", PublicPort: "http"}},
		},
	}
	expected := ra.DeepCopy()

	d := &RadixApplicationDefaulter{Annotation: annotation, OAuth2IssuerURL: issuerURL}
	require.NoError(t, d.Default(context.Background(), ra))
	assert.Equal(t, expected, ra)
}

func findComponent(t *testing.T, ra *radixv1.RadixApplication, name string) *radixv1.RadixComponent {
	t.Helper()
	for i := range ra.Spec.Components {
		if ra.Spec.Components[i].Name == name {
			return &ra.Spec.Components[i]
		}
	}
	require.FailNow(t, "component not found", name)
	return nil
}
