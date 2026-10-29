package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/equinor/radix-operator/easyauth/internal/defaulter"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

const (
	annotation = "radix.equinor.com/easyauth"
	issuerURL  = "https://easyauth.example.com"
	appName    = "my-app"
	appNs      = "my-app-app"
	devNs      = "my-app-dev"
	prodNs     = "my-app-prod"
)

var appRequest = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: appNs, Name: appName}}

func newRA(annotated bool) *radixv1.RadixApplication {
	ra := &radixv1.RadixApplication{
		Name:      appName,
		Namespace: appNs,
		Spec: radixv1.RadixApplicationSpec{
			Environments: []radixv1.Environment{{Name: "dev"}, {Name: "prod"}},
			Components: []radixv1.RadixComponent{
				{Name: "web", PublicPort: "http"},
				{Name: "legacy", Public: true},
				{Name: "api"},
			},
		},
	}
	if annotated {
		ra.Annotations = map[string]string{annotation: ""}
	}
	return ra
}

func oauthProxySecret(namespace, componentName string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		Name:      utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix),
		Namespace: namespace,
		Labels: map[string]string{
			kube.RadixAppLabel:                    appName,
			kube.RadixAuxiliaryComponentLabel:     componentName,
			kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType,
		},
		Data: data,
	}
}

func newRD(namespace, name string, components ...radixv1.RadixDeployComponent) *radixv1.RadixDeployment {
	return &radixv1.RadixDeployment{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{kube.RadixAppLabel: appName},
		Spec:      radixv1.RadixDeploymentSpec{AppName: appName, Components: components}, //nolint:staticcheck
	}
}

func newReconciler(objs ...client.Object) *RadixApplicationReconciler {
	s := scheme.NewScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return &RadixApplicationReconciler{
		Client:          c,
		APIReader:       c,
		Scheme:          s,
		Annotation:      annotation,
		OAuth2IssuerURL: issuerURL,
	}
}

func getSecret(t *testing.T, r *RadixApplicationReconciler, namespace, name string) *corev1.Secret {
	t.Helper()
	secret := &corev1.Secret{}
	require.NoError(t, r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, secret))
	return secret
}

func getRD(t *testing.T, r *RadixApplicationReconciler, namespace, name string) *radixv1.RadixDeployment {
	t.Helper()
	rd := &radixv1.RadixDeployment{}
	require.NoError(t, r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, rd))
	return rd
}

func TestReconcile_WithoutAnnotation_DoesNothing(t *testing.T) {
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")})
	rd := newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})
	r := newReconciler(newRA(false), webSecret, rd)

	result, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	err = r.Get(context.Background(), types.NamespacedName{Namespace: appNs, Name: ClientSecretName}, &corev1.Secret{})
	assert.True(t, kubeerrors.IsNotFound(err), "client secret must not be created")
	assert.NotContains(t, getSecret(t, r, devNs, webSecret.Name).Data, ClientSecretKey)
	assert.Nil(t, getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication)
}

func TestReconcile_RadixApplicationNotFound(t *testing.T) {
	r := newReconciler()
	_, err := r.Reconcile(context.Background(), appRequest)
	assert.NoError(t, err)
}

func TestReconcile_CreatesClientSecretAndSyncsToPublicComponents(t *testing.T) {
	ra := newRA(true)
	r := newReconciler(
		ra,
		oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")}),
		oauthProxySecret(devNs, "legacy", nil),
		oauthProxySecret(prodNs, "web", map[string][]byte{ClientSecretKey: []byte("stale")}),
		oauthProxySecret(devNs, "api", map[string][]byte{"CookieSecret": []byte("cookie")}),
		// Not an environment of the application
		oauthProxySecret("my-app-other", "web", nil),
	)

	result, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)
	assert.Equal(t, DefaultResyncInterval, result.RequeueAfter)

	clientSecret := getSecret(t, r, appNs, ClientSecretName)
	value := clientSecret.Data[ClientSecretKey]
	require.NotEmpty(t, value)
	assert.Equal(t, appName, clientSecret.Labels[kube.RadixAppLabel])
	require.Len(t, clientSecret.OwnerReferences, 1)
	assert.Equal(t, radixv1.KindRadixApplication, clientSecret.OwnerReferences[0].Kind)
	assert.Equal(t, appName, clientSecret.OwnerReferences[0].Name)

	webDev := getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name)
	assert.Equal(t, value, webDev.Data[ClientSecretKey])
	assert.Equal(t, []byte("cookie"), webDev.Data["CookieSecret"], "other keys must be kept")
	assert.Equal(t, value, getSecret(t, r, devNs, oauthProxySecret(devNs, "legacy", nil).Name).Data[ClientSecretKey])
	assert.Equal(t, value, getSecret(t, r, prodNs, oauthProxySecret(prodNs, "web", nil).Name).Data[ClientSecretKey])
	assert.NotContains(t, getSecret(t, r, devNs, oauthProxySecret(devNs, "api", nil).Name).Data, ClientSecretKey, "private components must not get the client secret")
	assert.NotContains(t, getSecret(t, r, "my-app-other", oauthProxySecret("my-app-other", "web", nil).Name).Data, ClientSecretKey)
}

func TestReconcile_ReusesExistingClientSecret(t *testing.T) {
	existing := &corev1.Secret{
		Name:      ClientSecretName,
		Namespace: appNs,
		Data:      map[string][]byte{ClientSecretKey: []byte("existing")},
	}
	webSecret := oauthProxySecret(devNs, "web", nil)
	r := newReconciler(newRA(true), existing, webSecret)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	clientSecret := getSecret(t, r, appNs, ClientSecretName)
	assert.Equal(t, []byte("existing"), clientSecret.Data[ClientSecretKey])
	assert.Equal(t, appName, clientSecret.Labels[kube.RadixAppLabel])
	assert.Len(t, clientSecret.OwnerReferences, 1)
	assert.Equal(t, []byte("existing"), getSecret(t, r, devNs, webSecret.Name).Data[ClientSecretKey])
}

func TestReconcile_GeneratesValueWhenClientSecretIsEmpty(t *testing.T) {
	existing := &corev1.Secret{Name: ClientSecretName, Namespace: appNs}
	r := newReconciler(newRA(true), existing)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.NotEmpty(t, getSecret(t, r, appNs, ClientSecretName).Data[ClientSecretKey])
}

func TestReconcile_SetsOAuth2ConfigOnAllRadixDeployments(t *testing.T) {
	expected := defaulter.OAuth2Config(appName, issuerURL)
	wrongOAuth2 := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "other"}}
	privateAuth := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "private"}}

	r := newReconciler(
		newRA(true),
		newRD(devNs, "dev-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", Authentication: wrongOAuth2.DeepCopy()},
			radixv1.RadixDeployComponent{Name: "legacy", Public: true}, //nolint:staticcheck
			radixv1.RadixDeployComponent{Name: "api", Authentication: privateAuth.DeepCopy()},
		),
		newRD(devNs, "dev-inactive", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
		newRD(prodNs, "prod-active", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
		newRD("my-app-other", "other", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	devActive := getRD(t, r, devNs, "dev-active")
	assert.Equal(t, expected, devActive.Spec.Components[0].Authentication.OAuth2)
	assert.Equal(t, expected, devActive.Spec.Components[1].Authentication.OAuth2)
	assert.Equal(t, privateAuth, devActive.Spec.Components[2].Authentication, "private components must not be changed")
	assert.Equal(t, expected, getRD(t, r, devNs, "dev-inactive").Spec.Components[0].Authentication.OAuth2)
	assert.Equal(t, expected, getRD(t, r, prodNs, "prod-active").Spec.Components[0].Authentication.OAuth2)
	assert.Nil(t, getRD(t, r, "my-app-other", "other").Spec.Components[0].Authentication, "RadixDeployments outside the app environments must not be changed")
}

func TestReconcile_IsIdempotent(t *testing.T) {
	r := newReconciler(
		newRA(true),
		oauthProxySecret(devNs, "web", nil),
		newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
	)
	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	secretVersion := getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name).ResourceVersion
	rdVersion := getRD(t, r, devNs, "dev-1").ResourceVersion

	_, err = r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.Equal(t, secretVersion, getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name).ResourceVersion)
	assert.Equal(t, rdVersion, getRD(t, r, devNs, "dev-1").ResourceVersion)
}

func TestMapToRadixApplication(t *testing.T) {
	requests := mapToRadixApplication(context.Background(), oauthProxySecret(devNs, "web", nil))
	assert.Equal(t, []ctrl.Request{appRequest}, requests)

	assert.Empty(t, mapToRadixApplication(context.Background(), &corev1.Secret{Name: "unlabeled", Namespace: devNs}))
}
