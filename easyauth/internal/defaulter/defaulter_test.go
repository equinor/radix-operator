package defaulter

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
)

const (
	annotation = "radix.equinor.com/easyauth"
	issuerURL  = "https://simpleauth.example.com"
	appID      = "01JABCDEFGHJKMNPQRSTVWXYZ0"
)

func newDefaulter(objs ...client.Object) *RadixApplicationDefaulter {
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(objs...).Build()
	return NewRadixApplicationDefaulter(c, annotation, issuerURL)
}

func newRR(appName, id string) *radixv1.RadixRegistration {
	rr := &radixv1.RadixRegistration{Name: appName}
	if id != "" {
		rr.Spec.AppID = radixv1.ULID{ULID: ulid.MustParse(id)}
	}
	return rr
}

func expectedOAuth2() *radixv1.OAuth2 {
	return &radixv1.OAuth2{
		ClientID:               "easyauth." + appID,
		Credentials:            radixv1.Secret,
		Scope:                  "openid profile email offline_access",
		SetAuthorizationHeader: new(false),
		SetXAuthRequestHeaders: new(false),
		SetUserHeaders:         new(true),
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

	d := newDefaulter(newRR("my-app", appID))
	require.NoError(t, d.Default(context.Background(), ra))

	for _, name := range []string{"public-port", "public-deprecated", "public-existing-oauth"} {
		component := findComponent(t, ra, name)
		require.NotNil(t, component.Authentication, name)
		assert.Equal(t, expectedOAuth2(), component.Authentication.OAuth2, name)
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

	// No RadixRegistration, since it is not read for RadixApplications without the annotation
	d := newDefaulter()
	require.NoError(t, d.Default(context.Background(), ra))
	assert.Equal(t, expected, ra)
}

func TestDefault_FailsWithoutAppID(t *testing.T) {
	tests := map[string][]client.Object{
		"RadixRegistration not found":     nil,
		"RadixRegistration without appID": {newRR("my-app", "")},
	}
	for name, objs := range tests {
		t.Run(name, func(t *testing.T) {
			ra := &radixv1.RadixApplication{
				Name:        "my-app",
				Annotations: map[string]string{annotation: ""},
				Spec: radixv1.RadixApplicationSpec{
					Components: []radixv1.RadixComponent{{Name: "web", PublicPort: "http"}},
				},
			}
			d := newDefaulter(objs...)
			assert.Error(t, d.Default(context.Background(), ra))
		})
	}
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
