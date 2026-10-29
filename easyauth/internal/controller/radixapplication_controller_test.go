package controller

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	api "github.com/dexidp/dex/api/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/equinor/radix-operator/easyauth/internal/defaulter"
	"github.com/equinor/radix-operator/easyauth/internal/dex"
	"github.com/equinor/radix-operator/easyauth/internal/dex/dextest"
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
	appID      = "01JABCDEFGHJKMNPQRSTVWXYZ0"
	clientID   = "easyauth." + appID

	clusterName     = "cluster"
	dnsZone         = "example.com"
	appAliasBaseURL = "app.example.com"
)

const rotationInterval = 90 * 24 * time.Hour

// newDexSecret returns a client secret created a day ago, so it is not rotated.
func newDexSecret() string {
	return dex.NewSecret(time.Now().Add(-24 * time.Hour))
}

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

func newRR() *radixv1.RadixRegistration {
	return &radixv1.RadixRegistration{Name: appName, Spec: radixv1.RadixRegistrationSpec{AppID: radixv1.ULID{ULID: ulid.MustParse(appID)}}}
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

func activeRD(rd *radixv1.RadixDeployment) *radixv1.RadixDeployment {
	rd.Status.Condition = radixv1.DeploymentActive
	return rd
}

func newReconciler(fakeDex *dextest.FakeDex, objs ...client.Object) *RadixApplicationReconciler {
	s := scheme.NewScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return newReconcilerWithClient(c, fakeDex)
}

func newReconcilerWithClient(c client.Client, fakeDex *dextest.FakeDex) *RadixApplicationReconciler {
	return NewRadixApplicationReconciler(c, dex.NewClientManager(fakeDex),
		annotation, issuerURL, clusterName, dnsZone, appAliasBaseURL, rotationInterval)
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
	fakeDex := dextest.NewFakeDex()
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")})
	rd := newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})
	r := newReconciler(fakeDex, newRA(false), newRR(), webSecret, rd)

	result, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	assert.Nil(t, fakeDex.Client(clientID), "Dex client must not be created")
	assert.Zero(t, fakeDex.Calls("GetClient"))
	assert.NotContains(t, getSecret(t, r, devNs, webSecret.Name).Data, ClientSecretKey)
	assert.Nil(t, getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication)
}

func TestReconcile_RadixApplicationNotFound(t *testing.T) {
	r := newReconciler(dextest.NewFakeDex())
	_, err := r.Reconcile(context.Background(), appRequest)
	assert.NoError(t, err)
}

func TestReconcile_FailsWithoutAppID(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	r := newReconciler(fakeDex, newRA(true), &radixv1.RadixRegistration{Name: appName})

	_, err := r.Reconcile(context.Background(), appRequest)
	assert.Error(t, err)
	assert.Zero(t, fakeDex.Calls("GetClient"))
}

func TestReconcile_CreatesDexClientWithRedirectURIs(t *testing.T) {
	ra := newRA(true)
	ra.Spec.DNSAlias = []radixv1.DNSAlias{
		{Alias: "my-alias", Environment: "prod", Component: "web"},
		// Private component
		{Alias: "private-alias", Environment: "prod", Component: "api"},
		// No active RadixDeployment in the environment
		{Alias: "other-alias", Environment: "other", Component: "web"},
	}
	fakeDex := dextest.NewFakeDex()
	r := newReconciler(fakeDex,
		ra,
		newRR(),
		activeRD(newRD(devNs, "dev-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", ExternalDNS: []radixv1.RadixDeployExternalDNS{{FQDN: "my-app.equinor.com"}}},
			radixv1.RadixDeployComponent{Name: "api"},
		)),
		// Inactive RadixDeployments are ignored
		newRD(devNs, "dev-inactive", radixv1.RadixDeployComponent{Name: "old", PublicPort: "http"}),
		activeRD(newRD(prodNs, "prod-active", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", DNSAppAlias: true})),
		// Not an environment of the application
		activeRD(newRD("my-app-other", "other", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
	)

	result, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)
	assert.Equal(t, DefaultResyncInterval, result.RequeueAfter)

	dexClient := fakeDex.Client(clientID)
	require.NotNil(t, dexClient)
	assert.Equal(t, appName, dexClient.Name)
	assert.NotEmpty(t, dexClient.Secret)
	assert.Equal(t, []string{
		// DNS alias
		"https://my-alias.example.com/oauth2/callback",
		// App alias
		"https://my-app.app.example.com/oauth2/callback",
		// External DNS
		"https://my-app.equinor.com/oauth2/callback",
		// Cluster and active cluster hosts
		"https://web-my-app-dev.cluster.example.com/oauth2/callback",
		"https://web-my-app-dev.example.com/oauth2/callback",
		"https://web-my-app-prod.cluster.example.com/oauth2/callback",
		"https://web-my-app-prod.example.com/oauth2/callback",
	}, dexClient.RedirectUris)
}

func TestConditionChangedPredicate(t *testing.T) {
	inactive := newRD(devNs, "dev-1")
	active := activeRD(newRD(devNs, "dev-1"))
	assert.True(t, conditionChangedPredicate.Update(event.UpdateEvent{ObjectOld: inactive, ObjectNew: active}))
	assert.True(t, conditionChangedPredicate.Update(event.UpdateEvent{ObjectOld: active, ObjectNew: inactive}))
	assert.False(t, conditionChangedPredicate.Update(event.UpdateEvent{ObjectOld: active, ObjectNew: active.DeepCopy()}))
}

func TestReconcile_SyncsDexSecretToPublicComponents(t *testing.T) {
	dexSecret := newDexSecret()
	fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret})
	r := newReconciler(fakeDex,
		newRA(true),
		newRR(),
		oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")}),
		oauthProxySecret(devNs, "legacy", nil),
		oauthProxySecret(prodNs, "web", map[string][]byte{ClientSecretKey: []byte("stale")}),
		oauthProxySecret(devNs, "api", map[string][]byte{"CookieSecret": []byte("cookie")}),
		// Not an environment of the application
		oauthProxySecret("my-app-other", "web", nil),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	value := []byte(dexSecret)
	webDev := getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name)
	assert.Equal(t, value, webDev.Data[ClientSecretKey])
	assert.Equal(t, []byte("cookie"), webDev.Data["CookieSecret"], "other keys must be kept")
	assert.Equal(t, value, getSecret(t, r, devNs, oauthProxySecret(devNs, "legacy", nil).Name).Data[ClientSecretKey])
	assert.Equal(t, value, getSecret(t, r, prodNs, oauthProxySecret(prodNs, "web", nil).Name).Data[ClientSecretKey])
	assert.NotContains(t, getSecret(t, r, devNs, oauthProxySecret(devNs, "api", nil).Name).Data, ClientSecretKey, "private components must not get the client secret")
	assert.NotContains(t, getSecret(t, r, "my-app-other", oauthProxySecret("my-app-other", "web", nil).Name).Data, ClientSecretKey)
	assert.Equal(t, dexSecret, fakeDex.Client(clientID).Secret, "the secret in Dex must not change")
}

func TestReconcile_DexUnavailable(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	fakeDex.SetError(status.Error(codes.Unavailable, "connection refused"))
	webSecret := oauthProxySecret(devNs, "web", nil)
	r := newReconciler(fakeDex, newRA(true), newRR(), webSecret,
		activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.Error(t, err)
	assert.NotContains(t, getSecret(t, r, devNs, webSecret.Name).Data, ClientSecretKey)
	assert.Equal(t, defaulter.OAuth2Config(clientID, issuerURL), getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2,
		"RadixDeployments must be protected even when Dex is unavailable")
}

func TestReconcile_PatchesRadixDeploymentsBeforeDex(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	dexCallsAtPatch := -1
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).
		WithObjects(newRA(true), newRR(), activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*radixv1.RadixDeployment); ok {
					dexCallsAtPatch = fakeDex.Calls("GetClient") + fakeDex.Calls("CreateClient") + fakeDex.Calls("UpdateClient")
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := newReconcilerWithClient(c, fakeDex)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)
	assert.Zero(t, dexCallsAtPatch, "RadixDeployment must be patched before Dex is called")
}

func TestReconcile_SyncsDexWhenRadixDeploymentPatchFails(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	webSecret := oauthProxySecret(devNs, "web", nil)
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).
		WithObjects(newRA(true), newRR(), webSecret, activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*radixv1.RadixDeployment); ok {
					return errors.New("patch failed")
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := newReconcilerWithClient(c, fakeDex)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.ErrorContains(t, err, "patch failed")
	dexClient := fakeDex.Client(clientID)
	require.NotNil(t, dexClient, "Dex client must be created even when the RadixDeployment patch fails")
	assert.Equal(t, []byte(dexClient.Secret), getSecret(t, r, devNs, webSecret.Name).Data[ClientSecretKey])
}

func TestReconcile_SetsOAuth2ConfigOnAllRadixDeployments(t *testing.T) {
	expected := defaulter.OAuth2Config(clientID, issuerURL)
	wrongOAuth2 := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "other"}}
	privateAuth := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "private"}}

	r := newReconciler(dextest.NewFakeDex(),
		newRA(true),
		newRR(),
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

func TestReconcile_RetriesRadixDeploymentPatchOnConflict(t *testing.T) {
	rd := newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})
	conflicts := 2
	patches := 0
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).
		WithObjects(newRA(true), newRR(), rd).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*radixv1.RadixDeployment); ok {
					patches++
					if patches <= conflicts {
						return kerrors.NewConflict(schema.GroupResource{Resource: "radixdeployments"}, obj.GetName(), errors.New("modified"))
					}
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := newReconcilerWithClient(c, dextest.NewFakeDex())

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.Equal(t, conflicts+1, patches)
	assert.Equal(t, defaulter.OAuth2Config(clientID, issuerURL), getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2)
}

func TestReconcile_FailsWhenRadixDeploymentPatchConflictsPersist(t *testing.T) {
	rd := newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})
	patches := 0
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).
		WithObjects(newRA(true), newRR(), rd).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*radixv1.RadixDeployment); ok {
					patches++
					return kerrors.NewConflict(schema.GroupResource{Resource: "radixdeployments"}, obj.GetName(), errors.New("modified"))
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := newReconcilerWithClient(c, dextest.NewFakeDex())

	_, err := r.Reconcile(context.Background(), appRequest)
	require.Error(t, err)
	var statusErr *kerrors.StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.True(t, kerrors.IsConflict(statusErr))
	assert.Equal(t, retry.DefaultRetry.Steps, patches)
}

func TestReconcile_IsIdempotent(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	r := newReconciler(fakeDex,
		newRA(true),
		newRR(),
		oauthProxySecret(devNs, "web", nil),
		oauthProxyDeployment(devNs, "web"),
		activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
	)
	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	deploymentVersion := getDeployment(t, r, devNs, "web").ResourceVersion
	secretVersion := getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name).ResourceVersion
	rdVersion := getRD(t, r, devNs, "dev-1").ResourceVersion
	dexSecret := fakeDex.Client(clientID).Secret

	_, err = r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.Equal(t, secretVersion, getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name).ResourceVersion)
	assert.Equal(t, rdVersion, getRD(t, r, devNs, "dev-1").ResourceVersion)
	assert.Equal(t, deploymentVersion, getDeployment(t, r, devNs, "web").ResourceVersion, "OAuth2 proxy must not be restarted")
	assert.Equal(t, dexSecret, fakeDex.Client(clientID).Secret)
	assert.Equal(t, 1, fakeDex.Calls("CreateClient"))
	assert.Zero(t, fakeDex.Calls("UpdateClient"))
}

func oauthProxyDeployment(namespace, componentName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name:      utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix),
		Namespace: namespace,
	}
}

func getDeployment(t *testing.T, r *RadixApplicationReconciler, namespace, componentName string) *appsv1.Deployment {
	t.Helper()
	deployment := &appsv1.Deployment{}
	require.NoError(t, r.Get(context.Background(), client.ObjectKeyFromObject(oauthProxyDeployment(namespace, componentName)), deployment))
	return deployment
}

func TestReconcile_RestartsOAuthProxyWhenClientSecretChanges(t *testing.T) {
	dexSecret := newDexSecret()
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte("old-secret")})
	r := newReconciler(dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret}),
		newRA(true), newRR(), webSecret, oauthProxyDeployment(devNs, "web"),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	secret := getSecret(t, r, devNs, webSecret.Name)
	assert.Equal(t, []byte(dexSecret), secret.Data[ClientSecretKey])
	assert.NotContains(t, secret.Annotations, RestartPendingAnnotation, "pending restart must be cleared after the restart")
	assert.NotEmpty(t, getDeployment(t, r, devNs, "web").Spec.Template.Annotations[RestartedAtAnnotation])
}

func TestReconcile_DoesNotRestartOAuthProxyWhenClientSecretIsMissing(t *testing.T) {
	dexSecret := newDexSecret()
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")})
	r := newReconciler(dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret}),
		newRA(true), newRR(), webSecret, oauthProxyDeployment(devNs, "web"),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.Equal(t, []byte(dexSecret), getSecret(t, r, devNs, webSecret.Name).Data[ClientSecretKey])
	assert.NotContains(t, getDeployment(t, r, devNs, "web").Spec.Template.Annotations, RestartedAtAnnotation)
}

func TestReconcile_RetriesPendingRestart(t *testing.T) {
	dexSecret := newDexSecret()
	// The client secret was synced, but the restart failed in a previous reconcile
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte(dexSecret)})
	webSecret.Annotations = map[string]string{RestartPendingAnnotation: "true"}
	r := newReconciler(dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret}),
		newRA(true), newRR(), webSecret, oauthProxyDeployment(devNs, "web"),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.NotContains(t, getSecret(t, r, devNs, webSecret.Name).Annotations, RestartPendingAnnotation)
	assert.NotEmpty(t, getDeployment(t, r, devNs, "web").Spec.Template.Annotations[RestartedAtAnnotation])
}

func TestReconcile_KeepsPendingRestartWhenRestartFails(t *testing.T) {
	dexSecret := newDexSecret()
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte("old-secret")})
	s := scheme.NewScheme()
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(newRA(true), newRR(), webSecret, oauthProxyDeployment(devNs, "web")).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*appsv1.Deployment); ok {
					return errors.New("patch failed")
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := newReconcilerWithClient(c, dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret}))

	_, err := r.Reconcile(context.Background(), appRequest)
	require.Error(t, err)

	secret := getSecret(t, r, devNs, webSecret.Name)
	assert.Equal(t, []byte(dexSecret), secret.Data[ClientSecretKey])
	assert.Contains(t, secret.Annotations, RestartPendingAnnotation, "pending restart must be kept so it is retried")
}

func TestReconcile_ClearsPendingRestartWithoutOAuthProxyDeployment(t *testing.T) {
	dexSecret := newDexSecret()
	webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte("old-secret")})
	r := newReconciler(dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret}),
		newRA(true), newRR(), webSecret,
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	secret := getSecret(t, r, devNs, webSecret.Name)
	assert.Equal(t, []byte(dexSecret), secret.Data[ClientSecretKey])
	assert.NotContains(t, secret.Annotations, RestartPendingAnnotation)
}

func TestReconcile_RotatesExpiredClientSecret(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expired := dex.NewSecret(time.Now().Add(-rotationInterval))
		fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: expired})
		webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte(expired)})
		r := newReconciler(fakeDex, newRA(true), newRR(), webSecret, oauthProxyDeployment(devNs, "web"))

		result, err := r.Reconcile(context.Background(), appRequest)
		require.NoError(t, err)

		rotated := fakeDex.Client(clientID).Secret
		assert.NotEqual(t, expired, rotated)
		createdAt, ok := dex.SecretCreatedAt(rotated)
		require.True(t, ok)
		assert.WithinDuration(t, time.Now(), createdAt, 0)
		assert.Equal(t, []byte(rotated), getSecret(t, r, devNs, webSecret.Name).Data[ClientSecretKey])
		assert.NotEmpty(t, getDeployment(t, r, devNs, "web").Spec.Template.Annotations[RestartedAtAnnotation], "OAuth2 proxy must be restarted with the rotated secret")
		assert.Equal(t, DefaultResyncInterval, result.RequeueAfter)
	})
}

func TestReconcile_RotatesClientSecretWithoutCreationTime(t *testing.T) {
	fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: "secret-from-an-older-version"})
	r := newReconciler(fakeDex, newRA(true), newRR())

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	rotated := fakeDex.Client(clientID).Secret
	assert.NotEqual(t, "secret-from-an-older-version", rotated)
	_, ok := dex.SecretCreatedAt(rotated)
	assert.True(t, ok, "rotated secret must have a creation time")
}

func TestReconcile_DoesNotRotateClientSecretBeforeInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notDue := dex.NewSecret(time.Now().Add(-rotationInterval + time.Second))
		fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: notDue})
		r := newReconciler(fakeDex, newRA(true), newRR())

		_, err := r.Reconcile(context.Background(), appRequest)
		require.NoError(t, err)

		assert.Equal(t, notDue, fakeDex.Client(clientID).Secret)
		assert.Zero(t, fakeDex.Calls("DeleteClient"))
	})
}

func TestNewRadixApplicationReconciler_Defaults(t *testing.T) {
	r := NewRadixApplicationReconciler(nil, nil, annotation, issuerURL, clusterName, dnsZone, appAliasBaseURL, 0)
	assert.Equal(t, DefaultResyncInterval, r.resyncInterval)
	assert.Equal(t, DefaultSecretRotationInterval, r.secretRotationInterval)
}

func TestReconcile_RequeuesAtRotationTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Rotated in 10 minutes, which is sooner than the resync interval
		secret := dex.NewSecret(time.Now().Add(-rotationInterval + 10*time.Minute))
		r := newReconciler(dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: secret}), newRA(true), newRR())

		result, err := r.Reconcile(context.Background(), appRequest)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, result.RequeueAfter, 10*time.Minute)
		assert.Less(t, result.RequeueAfter, 11*time.Minute)
	})
}

func TestMapToRadixApplication(t *testing.T) {
	requests := mapToRadixApplication(context.Background(), oauthProxySecret(devNs, "web", nil))
	assert.Equal(t, []ctrl.Request{appRequest}, requests)

	assert.Empty(t, mapToRadixApplication(context.Background(), &corev1.Secret{Name: "unlabeled", Namespace: devNs}))
}
