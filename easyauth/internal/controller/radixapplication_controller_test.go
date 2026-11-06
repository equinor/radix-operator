package controller

import (
	"context"
	"errors"
	"strings"
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

var oauth2Template = defaulter.NewOAuth2Template(radixv1.OAuth2{Scope: "openid profile email", SetUserHeaders: new(true)}, issuerURL)

const rotationInterval = 90 * 24 * time.Hour

// newDexSecret returns a client secret created a day ago, so it is not rotated.
func newDexSecret() string {
	return dex.NewSecret(time.Now().Add(-24 * time.Hour))
}

var appRequest = ctrl.Request{Namespace: appNs, Name: appName}

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
		Labels: map[string]string{
			kube.RadixAppLabel: appName,
			kube.RadixEnvLabel: strings.TrimPrefix(namespace, appName+"-"),
		},
		Spec: radixv1.RadixDeploymentSpec{AppName: appName, Components: components}, //nolint:staticcheck
	}
}

func activeRD(rd *radixv1.RadixDeployment) *radixv1.RadixDeployment {
	rd.Status.Condition = radixv1.DeploymentActive
	return rd
}

// webRD returns an active RadixDeployment in dev with the public web component
func webRD() *radixv1.RadixDeployment {
	return activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))
}

func newReconciler(fakeDex *dextest.FakeDex, objs ...client.Object) *RadixApplicationReconciler {
	s := scheme.NewScheme()
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return newReconcilerWithClient(c, fakeDex)
}

func newReconcilerWithClient(c client.Client, fakeDex *dextest.FakeDex) *RadixApplicationReconciler {
	return NewRadixApplicationReconciler(c, dex.NewClientManager(fakeDex),
		annotation, oauth2Template, clusterName, dnsZone, appAliasBaseURL, rotationInterval, 1)
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
	rd := activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))
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
	ra.Spec.Environments = append(ra.Spec.Environments, radixv1.Environment{Name: "other"})
	ra.Spec.DNSAlias = []radixv1.DNSAlias{
		{Alias: "my-alias", Environment: "prod", Component: "web"},
		// Private component
		{Alias: "private-alias", Environment: "prod", Component: "api"},
		// Alias for an orphaned environment, removed from the RadixApplication
		{Alias: "other-alias", Environment: "other", Component: "web"},
	}
	fakeDex := dextest.NewFakeDex()
	r := newReconciler(fakeDex,
		ra,
		newRR(),
		activeRD(newRD(devNs, "dev-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", ExternalDNS: []radixv1.RadixDeployExternalDNS{{FQDN: "latest.example.com"}, {FQDN: "my-app.equinor.com"}}},
			radixv1.RadixDeployComponent{Name: "api"},
		)),
		// Inactive RadixDeployments are ignored
		newRD(devNs, "dev-inactive", radixv1.RadixDeployComponent{Name: "old", PublicPort: "http"}),
		activeRD(newRD(prodNs, "prod-active", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", DNSAppAlias: true})),
		// An orphaned environment still has an active RadixDeployment
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
		// External DNS from the active RadixDeployment
		"https://latest.example.com/oauth2/callback",
		// DNS alias
		"https://my-alias.example.com/oauth2/callback",
		// App alias
		"https://my-app.app.example.com/oauth2/callback",
		// External DNS
		"https://my-app.equinor.com/oauth2/callback",
		// DNS alias from an environment without an active RadixDeployment
		"https://other-alias.example.com/oauth2/callback",
		// Cluster and active cluster hosts
		"https://web-my-app-dev.cluster.example.com/oauth2/callback",
		"https://web-my-app-dev.example.com/oauth2/callback",
		// Orphaned environment
		"https://web-my-app-other.cluster.example.com/oauth2/callback",
		"https://web-my-app-other.example.com/oauth2/callback",
		"https://web-my-app-prod.cluster.example.com/oauth2/callback",
		"https://web-my-app-prod.example.com/oauth2/callback",
	}, dexClient.RedirectUris)
}

func TestReconcile_RedirectURIsUseProxyPrefix(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	template := defaulter.NewOAuth2Template(radixv1.OAuth2{ProxyPrefix: "auth/"}, issuerURL)
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(newRA(true), newRR(), webRD()).Build()
	r := NewRadixApplicationReconciler(c, dex.NewClientManager(fakeDex),
		annotation, template, clusterName, dnsZone, appAliasBaseURL, rotationInterval, 1)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"https://web-my-app-dev.cluster.example.com/auth/callback",
		"https://web-my-app-dev.example.com/auth/callback",
	}, fakeDex.Client(clientID).RedirectUris)
	assert.Equal(t, "auth/", getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2.ProxyPrefix)
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
	ra := newRA(true)
	// removed is no longer in the RadixApplication, and private is no longer public, but both are still public in the
	// active RadixDeployment, so their OAuth2 proxies still run with the EasyAuth config
	ra.Spec.Components = append(ra.Spec.Components, radixv1.RadixComponent{Name: "private"})
	r := newReconciler(fakeDex,
		ra,
		newRR(),
		activeRD(newRD(devNs, "dev-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"},
			radixv1.RadixDeployComponent{Name: "legacy", PublicPort: "http"},
			radixv1.RadixDeployComponent{Name: "api"},
			radixv1.RadixDeployComponent{Name: "removed", PublicPort: "http"},
			radixv1.RadixDeployComponent{Name: "private", PublicPort: "http"},
		)),
		// Inactive RadixDeployments are ignored
		newRD(devNs, "dev-inactive", radixv1.RadixDeployComponent{Name: "old", PublicPort: "http"}),
		activeRD(newRD(prodNs, "prod-active", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
		activeRD(newRD("my-app-other", "other", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
		oauthProxySecret(devNs, "web", map[string][]byte{"CookieSecret": []byte("cookie")}),
		oauthProxySecret(devNs, "legacy", nil),
		oauthProxySecret(devNs, "removed", nil),
		oauthProxySecret(devNs, "private", nil),
		oauthProxySecret(devNs, "old", nil),
		oauthProxySecret(prodNs, "web", map[string][]byte{ClientSecretKey: []byte("stale")}),
		// Public in the RadixApplication, but not in the active RadixDeployment
		oauthProxySecret(prodNs, "legacy", nil),
		oauthProxySecret(devNs, "api", map[string][]byte{"CookieSecret": []byte("cookie")}),
		// Orphaned environment
		oauthProxySecret("my-app-other", "web", nil),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	value := []byte(dexSecret)
	webDev := getSecret(t, r, devNs, oauthProxySecret(devNs, "web", nil).Name)
	assert.Equal(t, value, webDev.Data[ClientSecretKey])
	assert.Equal(t, []byte("cookie"), webDev.Data["CookieSecret"], "other keys must be kept")
	assert.Equal(t, value, getSecret(t, r, devNs, oauthProxySecret(devNs, "legacy", nil).Name).Data[ClientSecretKey])
	assert.Equal(t, value, getSecret(t, r, devNs, oauthProxySecret(devNs, "removed", nil).Name).Data[ClientSecretKey])
	assert.Equal(t, value, getSecret(t, r, devNs, oauthProxySecret(devNs, "private", nil).Name).Data[ClientSecretKey])
	assert.Equal(t, value, getSecret(t, r, prodNs, oauthProxySecret(prodNs, "web", nil).Name).Data[ClientSecretKey])
	assert.NotContains(t, getSecret(t, r, devNs, oauthProxySecret(devNs, "old", nil).Name).Data, ClientSecretKey, "components of inactive RadixDeployments must not get the client secret")
	assert.NotContains(t, getSecret(t, r, prodNs, oauthProxySecret(prodNs, "legacy", nil).Name).Data, ClientSecretKey, "components not in the active RadixDeployment must not get the client secret")
	assert.NotContains(t, getSecret(t, r, devNs, oauthProxySecret(devNs, "api", nil).Name).Data, ClientSecretKey, "private components must not get the client secret")
	assert.Equal(t, value, getSecret(t, r, "my-app-other", oauthProxySecret("my-app-other", "web", nil).Name).Data[ClientSecretKey], "active orphaned deployments must receive the client secret")
	assert.Equal(t, dexSecret, fakeDex.Client(clientID).Secret, "the secret in Dex must not change")
}

func TestReconcile_CompletesSyncWhenCancelled(t *testing.T) {
	dexSecret := newDexSecret()
	fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: dexSecret})
	webSecret := oauthProxySecret(devNs, "web", nil)
	// Fails like the API server client does when the context is done
	failIfDone := func(ctx context.Context) error { return ctx.Err() }
	c := interceptor.NewClient(
		fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(newRA(true), newRR(), webSecret,
			activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))).Build(),
		interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.Join(failIfDone(ctx), c.Get(ctx, key, obj, opts...))
			},
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return errors.Join(failIfDone(ctx), c.List(ctx, list, opts...))
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if err := failIfDone(ctx); err != nil {
					return err
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		},
	)
	r := newReconcilerWithClient(c, fakeDex)

	// The manager cancels the context of running reconciles on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Reconcile(ctx, appRequest)
	require.NoError(t, err)

	assert.NotNil(t, fakeDex.Client(clientID))
	assert.Equal(t, []byte(dexSecret), getSecret(t, r, devNs, webSecret.Name).Data[ClientSecretKey])
	assert.Equal(t, defaulter.OAuth2Config(oauth2Template, clientID), getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2)
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
	assert.Equal(t, defaulter.OAuth2Config(oauth2Template, clientID), getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2,
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

func TestReconcile_SetsOAuth2ConfigOnActiveRadixDeployments(t *testing.T) {
	expected := defaulter.OAuth2Config(oauth2Template, clientID)
	wrongOAuth2 := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "other"}}
	privateAuth := &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "private"}}

	otherAppRD := activeRD(newRD("other-app-dev", "other-app", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))
	otherAppRD.Labels[kube.RadixAppLabel] = "other-app"
	otherAppRD.Spec.AppName = "other-app" //nolint:staticcheck

	r := newReconciler(dextest.NewFakeDex(),
		newRA(true),
		newRR(),
		activeRD(newRD(devNs, "dev-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", Authentication: wrongOAuth2.DeepCopy()},
			radixv1.RadixDeployComponent{Name: "legacy", PublicPort: "http"},
			radixv1.RadixDeployComponent{Name: "api", Authentication: privateAuth.DeepCopy()},
		)),
		newRD(devNs, "dev-inactive", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
		activeRD(newRD(prodNs, "prod-active", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"})),
		otherAppRD,
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	devActive := getRD(t, r, devNs, "dev-active")
	assert.Equal(t, expected, devActive.Spec.Components[0].Authentication.OAuth2)
	assert.Equal(t, expected, devActive.Spec.Components[1].Authentication.OAuth2)
	assert.Equal(t, privateAuth, devActive.Spec.Components[2].Authentication, "private components must not be changed")
	assert.Nil(t, getRD(t, r, devNs, "dev-inactive").Spec.Components[0].Authentication, "inactive RadixDeployments must not be changed")
	assert.Equal(t, expected, getRD(t, r, prodNs, "prod-active").Spec.Components[0].Authentication.OAuth2)
	assert.Nil(t, getRD(t, r, "other-app-dev", "other-app").Spec.Components[0].Authentication, "RadixDeployments of other apps must not be changed")
}

func TestReconcile_SetsOAuth2ConfigOnActiveRadixDeploymentsInOrphanedEnvironments(t *testing.T) {
	// The orphaned environment is removed from the RadixApplication, but its namespace and active RadixDeployment
	// still exist, so its OAuth2 proxy must keep using the EasyAuth config
	const orphanedNs = "my-app-orphaned"
	expected := defaulter.OAuth2Config(oauth2Template, clientID)

	r := newReconciler(dextest.NewFakeDex(),
		newRA(true),
		newRR(),
		activeRD(newRD(orphanedNs, "orphaned-active",
			radixv1.RadixDeployComponent{Name: "web", PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "other"}}},
			radixv1.RadixDeployComponent{Name: "api"},
		)),
		newRD(orphanedNs, "orphaned-inactive", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	orphanedActive := getRD(t, r, orphanedNs, "orphaned-active")
	assert.Equal(t, expected, orphanedActive.Spec.Components[0].Authentication.OAuth2, "active RadixDeployments in orphaned environments must be synced")
	assert.Nil(t, orphanedActive.Spec.Components[1].Authentication, "private components must not be changed")
	assert.Nil(t, getRD(t, r, orphanedNs, "orphaned-inactive").Spec.Components[0].Authentication, "inactive RadixDeployments must not be changed")
}

func TestReconcile_RetriesRadixDeploymentPatchOnConflict(t *testing.T) {
	rd := activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))
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
	assert.Equal(t, defaulter.OAuth2Config(oauth2Template, clientID), getRD(t, r, devNs, "dev-1").Spec.Components[0].Authentication.OAuth2)
}

func TestReconcile_FailsWhenRadixDeploymentPatchConflictsPersist(t *testing.T) {
	rd := activeRD(newRD(devNs, "dev-1", radixv1.RadixDeployComponent{Name: "web", PublicPort: "http"}))
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
		newRA(true), newRR(), webSecret, webRD(), oauthProxyDeployment(devNs, "web"),
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
		newRA(true), newRR(), webSecret, webRD(), oauthProxyDeployment(devNs, "web"),
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
		newRA(true), newRR(), webSecret, webRD(), oauthProxyDeployment(devNs, "web"),
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
		WithObjects(newRA(true), newRR(), webSecret, webRD(), oauthProxyDeployment(devNs, "web")).
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
		newRA(true), newRR(), webSecret, webRD(),
	)

	_, err := r.Reconcile(context.Background(), appRequest)
	require.NoError(t, err)

	secret := getSecret(t, r, devNs, webSecret.Name)
	assert.Equal(t, []byte(dexSecret), secret.Data[ClientSecretKey])
	assert.NotContains(t, secret.Annotations, RestartPendingAnnotation)
}

func TestReconcile_RotatesExpiredClientSecret(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expired := dex.NewSecret(time.Now().Add(-rotationInterval - time.Second))
		fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: appName, Secret: expired})
		webSecret := oauthProxySecret(devNs, "web", map[string][]byte{ClientSecretKey: []byte(expired)})
		r := newReconciler(fakeDex, newRA(true), newRR(), webSecret, webRD(), oauthProxyDeployment(devNs, "web"))

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
	r := NewRadixApplicationReconciler(nil, nil, annotation, oauth2Template, clusterName, dnsZone, appAliasBaseURL, 0, 1)
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
