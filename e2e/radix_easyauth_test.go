package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// Must match annotations.EasyAuth and annotations.AllowedIdentities in easyauth/internal/annotations
	easyAuthAnnotation                  = "easyauth.radix.equinor.com"
	easyAuthAllowedIdentitiesAnnotation = "easyauth.radix.equinor.com/allowed-identities"
	// Must match config.oauth2.allowedGroups in testdata/radix-easyauth-values.yaml
	easyAuthDefaultGroup = "e2e-default-group"
	// Must match the allowed identities annotation in testdata/gitserver/repos/equinor/easyauth-test.git
	easyAuthTestAppGroup = "e2e-group"
	// Must match config.issuerUrl in testdata/radix-easyauth-values.yaml
	easyAuthOAuth2IssuerURL = "http://radix-easyauth-dex.radix-easyauth.svc:5556"
	// Must match the static user in dex.config in testdata/radix-easyauth-values.yaml
	easyAuthTestUser     = "e2e-user@example.com"
	easyAuthTestPassword = "e2e-password"
	// A static user that is not in easyAuthTestAppGroup
	easyAuthOtherTestUser = "e2e-other-user@example.com"
	// Must match annotations.RestartPending and annotations.RestartedAt in easyauth/internal/annotations
	easyAuthRestartPendingAnnotation = "easyauth.radix.equinor.com/restart-pending"
	easyAuthRestartedAtAnnotation    = "easyauth.radix.equinor.com/restartedAt"
	// Shown by the web component in testdata/gitserver/repos/equinor/easyauth-test.git
	easyAuthWebContent = "Welcome to nginx!"
)

// TestRadixEasyAuthMutatingWebhook verifies that the radix-easyauth mutating webhook only mutates
// RadixApplications that have the easyauth annotation
func TestRadixEasyAuthMutatingWebhook(t *testing.T) {
	t.Parallel()
	c := getClient(t)

	t.Run("overrides OAuth2 config on public components when RadixApplication has easyauth annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-oauth2"
		appID := createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, map[string]string{easyAuthAnnotation: ""})
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		// Create updates ra with the object persisted by the API server, after mutation
		assertEasyAuthOAuth2(t, ra, appID)
	})

	t.Run("sets allowed groups from the allowed identities annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-groups"
		appID := createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, map[string]string{easyAuthAnnotation: "", easyAuthAllowedIdentitiesAnnotation: "group-a,group-b"})
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		assertEasyAuthOAuth2(t, ra, appID, "group-a", "group-b")
	})

	t.Run("rejects RadixApplication with invalid easyauth annotations", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-invalid"
		createEasyAuthTestApp(t, c, appName)

		for _, annotations := range []map[string]string{
			{easyAuthAnnotation: "group-a"},
			{easyAuthAnnotation: "", easyAuthAllowedIdentitiesAnnotation: "group-a,,group-b"},
			{easyAuthAnnotation: "", easyAuthAllowedIdentitiesAnnotation: "group-a, group-b"},
		} {
			ra := newEasyAuthTestRA(appName, annotations)
			assert.Error(t, c.Create(t.Context(), ra), "annotations %v should be rejected", annotations)
		}
	})

	t.Run("overrides user changes to OAuth2 config on update", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-oauth2-update"
		appID := createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, map[string]string{easyAuthAnnotation: ""})
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		key := client.ObjectKeyFromObject(ra)
		require.NoError(t, retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latest := &v1.RadixApplication{}
			if err := testManager.GetAPIReader().Get(t.Context(), key, latest); err != nil {
				return err
			}
			web := &latest.Spec.Components[0]
			web.Authentication.OAuth2.ClientID = "user-changed"
			web.Authentication.OAuth2.SkipAuthRoutes = []string{"^/healthz$"}
			web.Authentication.OAuth2.OIDC.IssuerURL = "https://login.example.com"
			web.EnvironmentConfig[0].Authentication = &v1.Authentication{
				OAuth2: &v1.OAuth2{ClientID: "env-changed"},
			}
			return c.Update(t.Context(), latest)
		}))

		persisted := &v1.RadixApplication{}
		require.NoError(t, testManager.GetAPIReader().Get(t.Context(), key, persisted))
		assertEasyAuthOAuth2(t, persisted, appID)
	})

	t.Run("does not change OAuth2 config when RadixApplication lacks easyauth annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-not-annotated"
		createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, nil)
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		web := ra.Spec.Components[0]
		require.NotNil(t, web.Authentication)
		assert.Equal(t, "user-defined", web.Authentication.OAuth2.ClientID)
		require.NotNil(t, web.EnvironmentConfig[0].Authentication)
		assert.Equal(t, "env-defined", web.EnvironmentConfig[0].Authentication.OAuth2.ClientID)
	})
}

// newEasyAuthTestRA returns a RadixApplication with a public component "web" and a private
// component "api", where "web" has user defined OAuth2 config on both component and environment level
func newEasyAuthTestRA(appName string, annotations map[string]string) *v1.RadixApplication {
	return &v1.RadixApplication{
		Name:        appName,
		Namespace:   appName + "-app",
		Annotations: annotations,
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{{Name: "dev"}},
			Components: []v1.RadixComponent{
				{
					Name:       "web",
					Ports:      []v1.ComponentPort{{Name: "http", Port: 8080}},
					PublicPort: "http",
					Authentication: &v1.Authentication{
						OAuth2: &v1.OAuth2{
							ClientID: "user-defined",
							Scope:    "openid",
							OIDC:     &v1.OAuth2OIDC{IssuerURL: "https://login.example.com"},
						},
					},
					EnvironmentConfig: []v1.RadixEnvironmentConfig{{
						Environment: "dev",
						Authentication: &v1.Authentication{
							OAuth2: &v1.OAuth2{ClientID: "env-defined"},
						},
					}},
				},
				{
					Name:  "api",
					Ports: []v1.ComponentPort{{Name: "http", Port: 8080}},
				},
			},
		},
	}
}

// assertEasyAuthOAuth2 asserts that the public component "web" only has the easyauth OAuth2 config,
// and that the private component "api" is unchanged
func assertEasyAuthOAuth2(t *testing.T, ra *v1.RadixApplication, appID v1.ULID, allowedGroups ...string) {
	t.Helper()
	expected := expectedEasyAuthOAuth2(appID, allowedGroups...)
	web, api := ra.Spec.Components[0], ra.Spec.Components[1]
	require.NotNil(t, web.Authentication)
	assert.Equal(t, expected, web.Authentication.OAuth2)
	require.Len(t, web.EnvironmentConfig, 1)
	if web.EnvironmentConfig[0].Authentication != nil {
		assert.Nil(t, web.EnvironmentConfig[0].Authentication.OAuth2, "environment OAuth2 config should be removed")
	}
	assert.Nil(t, api.Authentication, "private components should not be changed")
}

// expectedEasyAuthOAuth2 returns the OAuth2 config radix-easyauth sets on public components. allowedGroups defaults
// to easyAuthDefaultGroup.
func expectedEasyAuthOAuth2(appID v1.ULID, allowedGroups ...string) *v1.OAuth2 {
	if len(allowedGroups) == 0 {
		allowedGroups = []string{easyAuthDefaultGroup}
	}
	return &v1.OAuth2{
		AllowedGroups:          allowedGroups,
		ClientID:               "easyauth." + appID.String(),
		Credentials:            v1.Secret,
		Scope:                  "openid profile email groups offline_access",
		SetAuthorizationHeader: new(false),
		SetXAuthRequestHeaders: new(false),
		SetUserHeaders:         new(true),
		SessionStoreType:       v1.SessionStoreSystemManaged,
		ProxyPrefix:            "/oauth2",
		OIDC:                   &v1.OAuth2OIDC{IssuerURL: easyAuthOAuth2IssuerURL},
	}
}

// TestRadixEasyAuthPipeline verifies that RadixDeployments created by Deploy and BuildDeploy
// pipeline jobs contain the OAuth2 config set by radix-easyauth, and not the user defined OAuth2
// config from radixconfig.yaml (testdata/gitserver/repos/equinor/easyauth-test.git).
// The Deploy job creates the RadixApplication and the BuildDeploy job updates it, which covers
// both admission paths in the pipeline apply-config step.
func TestRadixEasyAuthPipeline(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const (
		appName     = "easyauth-test"
		envName     = "dev"
		timeout     = 5 * time.Minute
		waitTimeout = 60 * time.Second
	)

	appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	rr := &v1.RadixRegistration{}
	require.NoError(t, testManager.GetAPIReader().Get(t.Context(), client.ObjectKey{Name: appName}, rr))
	appID := rr.Spec.AppID
	require.NoError(t, waitForSecret(t.Context(), c, appNamespace, defaults.GitPrivateKeySecretName, waitTimeout),
		"registration secret %s should be created in %s", defaults.GitPrivateKeySecretName, appNamespace)
	require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, waitTimeout), "pipeline RBAC should be provisioned in %s", appNamespace)

	jobs := []*v1.RadixJob{
		{
			Name: "easyauth-deploy", Namespace: appNamespace,
			Spec: v1.RadixJobSpec{
				AppName:      appName,
				PipeLineType: v1.Deploy,
				Deploy:       v1.RadixDeploySpec{ToEnvironment: envName},
				TriggeredBy:  "e2e",
			},
		},
		{
			Name: "easyauth-build-deploy", Namespace: appNamespace,
			Spec: v1.RadixJobSpec{
				AppName:      appName,
				PipeLineType: v1.BuildDeploy,
				Build:        v1.RadixBuildSpec{Branch: envName, GitRef: envName, GitRefType: v1.GitRefBranch}, //nolint:staticcheck
				TriggeredBy:  "e2e",
			},
		},
	}

	for _, rj := range jobs {
		require.NoError(t, c.Create(t.Context(), rj), "should create job %s", rj.Name)
		cond, err := waitForJobCondition(t.Context(), c, appNamespace, rj.Name, v1.RadixJobCondition.IsDone, timeout)
		if err != nil || cond != v1.JobSucceeded {
			logRadixJobDiagnostics(t, c, appNamespace, rj.Name)
			logRadixOperatorLogs(t, c, appName)
		}
		require.NoError(t, err, "job %s should finish, last condition: %s", rj.Name, cond)
		require.Equal(t, v1.JobSucceeded, cond, "job %s should succeed", rj.Name)

		ra := &v1.RadixApplication{}
		require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: appNamespace, Name: appName}, ra))
		assertEasyAuthOAuth2(t, ra, appID, easyAuthTestAppGroup)

		rds := &v1.RadixDeploymentList{}
		require.NoError(t, c.List(t.Context(), rds,
			client.InNamespace(utils.GetEnvironmentNamespace(appName, envName)),
			client.MatchingLabels{kube.RadixJobNameLabel: rj.Name}))
		require.Len(t, rds.Items, 1, "job %s should create one RadixDeployment", rj.Name)
		rd := &rds.Items[0]

		web := rd.GetComponentByName("web")
		require.NotNil(t, web, "RadixDeployment %s should contain component web", rd.Name)
		require.NotNil(t, web.Authentication, "RadixDeployment %s component web should have authentication", rd.Name)
		assert.Equal(t, expectedEasyAuthOAuth2(appID, easyAuthTestAppGroup), web.Authentication.OAuth2, "RadixDeployment %s component web should have the radix-easyauth OAuth2 config", rd.Name)

		api := rd.GetComponentByName("api")
		require.NotNil(t, api, "RadixDeployment %s should contain component api", rd.Name)
		assert.Nil(t, api.Authentication, "RadixDeployment %s component api should not have authentication", rd.Name)
	}

	// The OAuth2 proxy Secret is created by radix-operator, and the client secret is written by the radix-easyauth
	// controller after it has created the client in Dex
	envNamespace := utils.GetEnvironmentNamespace(appName, envName)
	allowOAuth2ProxyEgressToDex(t, c, envNamespace)
	oauthSecretName := utils.GetAuxiliaryComponentSecretName("web", v1.OAuthProxyAuxiliaryComponentSuffix)
	var dexSecret []byte
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		secret := &corev1.Secret{}
		if !assert.NoError(collect, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: oauthSecretName}, secret)) {
			return
		}
		dexSecret = secret.Data[defaults.OAuthClientSecretKeyName]
		assert.NotEmpty(collect, dexSecret)
	}, waitTimeout, time.Second, "OAuth2 proxy Secret %s/%s should get the client secret from Dex", envNamespace, oauthSecretName)

	// A changed client secret is reverted to the secret from Dex, and the OAuth2 proxy is restarted to pick it up
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: oauthSecretName}, secret))
	original := secret.DeepCopy()
	secret.Data[defaults.OAuthClientSecretKeyName] = []byte("changed-by-user")
	require.NoError(t, c.Patch(t.Context(), secret, client.MergeFrom(original)))

	oauthDeploymentName := utils.GetAuxiliaryComponentDeploymentName("web", v1.OAuthProxyAuxiliaryComponentSuffix)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		secret := &corev1.Secret{}
		if !assert.NoError(collect, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: oauthSecretName}, secret)) {
			return
		}
		assert.Equal(collect, dexSecret, secret.Data[defaults.OAuthClientSecretKeyName])
		assert.NotContains(collect, secret.Annotations, easyAuthRestartPendingAnnotation)

		deployment := &appsv1.Deployment{}
		if !assert.NoError(collect, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: oauthDeploymentName}, deployment)) {
			return
		}
		assert.Contains(collect, deployment.Spec.Template.Annotations, easyAuthRestartedAtAnnotation)
	}, waitTimeout, time.Second, "OAuth2 proxy %s/%s should be restarted with the client secret from Dex", envNamespace, oauthDeploymentName)

	// Log in through Dex with the static user, and reach the web component through the OAuth2 proxy
	readyCtx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	if err := WaitForDeploymentReady(readyCtx, c, envNamespace, oauthDeploymentName); err != nil {
		logPodLogs(t, envNamespace, fmt.Sprintf("%s=%s", kube.RadixAuxiliaryComponentTypeLabel, v1.OAuthProxyAuxiliaryComponentType), "")
		require.NoError(t, err, "OAuth2 proxy %s/%s should be ready: %s", envNamespace, oauthDeploymentName, describeDeploymentPods(t.Context(), c, envNamespace, oauthDeploymentName))
	}
	browser := newEasyAuthBrowser(t, c, envNamespace, "web", oauthDeploymentName)

	t.Run("static user logs in with Dex and reaches the web component", func(t *testing.T) {
		client := browser.newClient(t)
		loginURL := browser.openLoginForm(t, client)

		resp, body := browser.submitLogin(t, client, loginURL, easyAuthTestUser, easyAuthTestPassword)
		require.Equal(t, http.StatusOK, resp.StatusCode, "login should succeed, body: %s", body)
		assert.Equal(t, browser.webURL.Host, resp.Request.URL.Host, "login should redirect back to the web component")
		assert.Contains(t, body, easyAuthWebContent)

		// The session cookie set by the OAuth2 proxy authenticates later requests without Dex
		resp, body = browser.get(t, withoutRedirects(client), browser.webURL)
		require.Equal(t, http.StatusOK, resp.StatusCode, "authenticated request should reach the web component, body: %s", body)
		assert.Contains(t, body, easyAuthWebContent)
	})

	t.Run("static user outside the allowed groups is rejected by the OAuth2 proxy", func(t *testing.T) {
		client := browser.newClient(t)
		loginURL := browser.openLoginForm(t, client)

		resp, body := browser.submitLogin(t, client, loginURL, easyAuthOtherTestUser, easyAuthTestPassword)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "body: %s", body)
		assert.NotContains(t, body, easyAuthWebContent)

		resp, body = browser.get(t, withoutRedirects(client), browser.webURL)
		browser.assertRedirectedToDex(t, resp)
		assert.NotContains(t, body, easyAuthWebContent)
	})

	t.Run("unauthenticated request is redirected to Dex", func(t *testing.T) {
		resp, body := browser.get(t, withoutRedirects(browser.newClient(t)), browser.webURL)
		browser.assertRedirectedToDex(t, resp)
		assert.NotContains(t, body, easyAuthWebContent)
	})

	t.Run("forged session cookie is rejected", func(t *testing.T) {
		client := withoutRedirects(browser.newClient(t))
		client.Jar.SetCookies(browser.webURL, []*http.Cookie{{Name: "_oauth2_proxy", Value: "forged", Secure: true}})
		resp, body := browser.get(t, client, browser.webURL)
		browser.assertRedirectedToDex(t, resp)
		assert.NotContains(t, body, easyAuthWebContent)
	})

	t.Run("invalid password is rejected by Dex", func(t *testing.T) {
		client := browser.newClient(t)
		loginURL := browser.openLoginForm(t, client)

		resp, body := browser.submitLogin(t, client, loginURL, easyAuthTestUser, "wrong-password")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body: %s", body)
		assert.Equal(t, browser.issuerURL.Host, resp.Request.URL.Host, "Dex should not redirect back to the web component")
		assert.NotContains(t, body, easyAuthWebContent)

		resp, body = browser.get(t, withoutRedirects(client), browser.webURL)
		browser.assertRedirectedToDex(t, resp)
		assert.NotContains(t, body, easyAuthWebContent)
	})

	t.Run("unknown user is rejected by Dex", func(t *testing.T) {
		client := browser.newClient(t)
		loginURL := browser.openLoginForm(t, client)

		resp, body := browser.submitLogin(t, client, loginURL, "unknown@example.com", easyAuthTestPassword)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body: %s", body)
		assert.Equal(t, browser.issuerURL.Host, resp.Request.URL.Host, "Dex should not redirect back to the web component")
		assert.NotContains(t, body, easyAuthWebContent)
	})

	if t.Failed() {
		logPodLogs(t, envNamespace, fmt.Sprintf("%s=%s", kube.RadixAuxiliaryComponentTypeLabel, v1.OAuthProxyAuxiliaryComponentType), "")
		logPodLogs(t, easyAuthNamespace, "app.kubernetes.io/component=dex", "")
	}
}

// TestRadixEasyAuthPromoteOldDeployment verifies that promoting an old RadixDeployment, created before EasyAuth was
// activated, results in a RadixDeployment with the OAuth2 config set by radix-easyauth. The radix-easyauth controller
// only syncs active RadixDeployments, so the old inactive RadixDeployment never gets the OAuth2 config.
// EasyAuth is activated by pointing the RadixRegistration from radixconfig-before-easyauth.yaml to radixconfig.yaml
// (testdata/gitserver/repos/equinor/easyauth-promote-test.git), which has the easyauth annotation.
func TestRadixEasyAuthPromoteOldDeployment(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const (
		appName     = "easyauth-promote-test"
		waitTimeout = 60 * time.Second
	)

	appNamespace := appName + "-app"
	devNamespace := utils.GetEnvironmentNamespace(appName, "dev")
	prodNamespace := utils.GetEnvironmentNamespace(appName, "prod")
	createNamespaceForTest(t, c, appName)
	rr := &v1.RadixRegistration{
		Name: appName,
		Spec: v1.RadixRegistrationSpec{
			AppID:               v1.ULID{ULID: ulid.Make()},
			CloneURL:            "git@github.com:equinor/" + appName + ".git",
			AdGroups:            []string{"test-group"},
			ConfigBranch:        "main",
			ConfigurationItem:   "test-item",
			RadixConfigFullName: "radixconfig-before-easyauth.yaml",
		},
	}
	require.NoError(t, c.Create(t.Context(), rr))
	appID := rr.Spec.AppID
	require.NoError(t, waitForSecret(t.Context(), c, appNamespace, defaults.GitPrivateKeySecretName, waitTimeout),
		"registration secret %s should be created in %s", defaults.GitPrivateKeySecretName, appNamespace)
	require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, waitTimeout), "pipeline RBAC should be provisioned in %s", appNamespace)

	// Deploy twice before EasyAuth is activated, so the first RadixDeployment becomes inactive
	oldRD := runEasyAuthTestJob(t, c, appName, "easyauth-promote-deploy-old", devNamespace,
		v1.RadixJobSpec{PipeLineType: v1.Deploy, Deploy: v1.RadixDeploySpec{ToEnvironment: "dev"}})
	newRD := runEasyAuthTestJob(t, c, appName, "easyauth-promote-deploy-new", devNamespace,
		v1.RadixJobSpec{PipeLineType: v1.Deploy, Deploy: v1.RadixDeploySpec{ToEnvironment: "dev"}})
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		for _, rd := range []*v1.RadixDeployment{oldRD, newRD} {
			assert.NoError(collect, c.Get(t.Context(), client.ObjectKeyFromObject(rd), rd))
		}
		assert.Equal(collect, v1.DeploymentInactive, oldRD.Status.Condition)
		assert.Equal(collect, v1.DeploymentActive, newRD.Status.Condition)
	}, waitTimeout, time.Second, "RadixDeployment %s should be replaced by %s", oldRD.Name, newRD.Name)
	web := oldRD.GetComponentByName("web")
	require.NotNil(t, web, "RadixDeployment %s should contain component web", oldRD.Name)
	require.Nil(t, web.Authentication, "sanity check: RadixDeployment %s created before EasyAuth should not have authentication", oldRD.Name)

	// Activate EasyAuth. The promote pipeline applies radixconfig.yaml with the easyauth annotation.
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(rr), rr))
	original := rr.DeepCopy()
	rr.Spec.RadixConfigFullName = "radixconfig.yaml"
	require.NoError(t, c.Patch(t.Context(), rr, client.MergeFrom(original)))

	promotedRD := runEasyAuthTestJob(t, c, appName, "easyauth-promote", prodNamespace,
		v1.RadixJobSpec{PipeLineType: v1.Promote, Promote: v1.RadixPromoteSpec{
			DeploymentName: oldRD.Name, FromEnvironment: "dev", ToEnvironment: "prod",
		}})
	assert.Equal(t, oldRD.Name, promotedRD.Annotations[kube.RadixDeploymentPromotedFromDeploymentAnnotation])

	ra := &v1.RadixApplication{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: appNamespace, Name: appName}, ra))
	require.Contains(t, ra.Annotations, easyAuthAnnotation, "the promote pipeline should apply radixconfig.yaml")

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		rd := &v1.RadixDeployment{}
		if !assert.NoError(collect, c.Get(t.Context(), client.ObjectKeyFromObject(promotedRD), rd)) {
			return
		}
		assert.Equal(collect, v1.DeploymentActive, rd.Status.Condition)
		web := rd.GetComponentByName("web")
		if assert.NotNil(collect, web) && assert.NotNil(collect, web.Authentication) {
			assert.Equal(collect, expectedEasyAuthOAuth2(appID), web.Authentication.OAuth2)
		}
	}, waitTimeout, time.Second, "promoted RadixDeployment %s should be active with the radix-easyauth OAuth2 config", promotedRD.Name)

	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(oldRD), oldRD))
	assert.Nil(t, oldRD.GetComponentByName("web").Authentication, "inactive RadixDeployment %s should not be changed", oldRD.Name)
}

// runEasyAuthTestJob creates a RadixJob, waits for it to succeed, and returns the RadixDeployment it created in envNamespace
func runEasyAuthTestJob(t *testing.T, c client.Client, appName, jobName, envNamespace string, spec v1.RadixJobSpec) *v1.RadixDeployment {
	t.Helper()
	const timeout = 5 * time.Minute
	appNamespace := appName + "-app"
	spec.AppName = appName
	spec.TriggeredBy = "e2e"
	rj := &v1.RadixJob{Name: jobName, Namespace: appNamespace, Spec: spec}
	require.NoError(t, c.Create(t.Context(), rj), "should create job %s", rj.Name)
	cond, err := waitForJobCondition(t.Context(), c, appNamespace, rj.Name, v1.RadixJobCondition.IsDone, timeout)
	if err != nil || cond != v1.JobSucceeded {
		logRadixJobDiagnostics(t, c, appNamespace, rj.Name)
		logRadixOperatorLogs(t, c, appName)
	}
	require.NoError(t, err, "job %s should finish, last condition: %s", rj.Name, cond)
	require.Equal(t, v1.JobSucceeded, cond, "job %s should succeed", rj.Name)

	rds := &v1.RadixDeploymentList{}
	require.NoError(t, c.List(t.Context(), rds, client.InNamespace(envNamespace), client.MatchingLabels{kube.RadixJobNameLabel: rj.Name}))
	require.Len(t, rds.Items, 1, "job %s should create one RadixDeployment", rj.Name)
	return &rds.Items[0]
}

// allowOAuth2ProxyEgressToDex allows the OAuth2 proxies in namespace to reach the Dex HTTP port. radix-operator only
// allows OAuth2 proxy egress to port 443, which is where the issuer is reached through the gateway in a Radix cluster.
// The e2e issuer is the Dex Service, since there is no gateway controller in the kind cluster.
// The policy has no radix labels, so radix-operator leaves it alone.
func allowOAuth2ProxyEgressToDex(t *testing.T, c client.Client, namespace string) {
	t.Helper()
	tcp, dexHTTPPort := corev1.ProtocolTCP, intstr.FromInt32(5556)
	policy := &networkingv1.NetworkPolicy{
		Name:      "e2e-allow-oauth-aux-egress-to-dex",
		Namespace: namespace,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
				kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType,
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: easyAuthNamespace}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/component": "dex"}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &dexHTTPPort}},
			}},
		},
	}
	require.NoError(t, client.IgnoreAlreadyExists(c.Create(t.Context(), policy)))
}

// easyAuthBrowser logs in like a browser, through port forwards to the OAuth2 proxy and Dex pods, since there is
// no gateway controller in the kind cluster. Requests keep their public URLs, so redirects, redirect URIs and
// Secure cookies work as they would through the gateway.
type easyAuthBrowser struct {
	webURL    *url.URL
	issuerURL *url.URL
	transport http.RoundTripper
}

func newEasyAuthBrowser(t *testing.T, c client.Client, envNamespace, componentName, oauthDeploymentName string) *easyAuthBrowser {
	t.Helper()
	issuerURL, err := url.Parse(easyAuthOAuth2IssuerURL)
	require.NoError(t, err)
	issuerPort, err := strconv.Atoi(issuerURL.Port())
	require.NoError(t, err, "issuer URL %s should have a port", issuerURL)

	// EasyAuth builds the same hosts as radix-operator does for the HTTPRoute, so every hostname has a redirect URI in Dex
	route := &gatewayapiv1.HTTPRoute{}
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		if assert.NoError(collect, c.Get(t.Context(), client.ObjectKey{Namespace: envNamespace, Name: componentName}, route)) {
			assert.NotEmpty(collect, route.Spec.Hostnames)
		}
	}, time.Minute, time.Second, "HTTPRoute %s/%s should have hostnames", envNamespace, componentName)
	webURL := &url.URL{Scheme: "https", Host: string(route.Spec.Hostnames[0]), Path: "/"}

	cfg := testManager.GetConfig()
	proxyAddr, err := internal.PortForward(t.Context(), cfg, envNamespace, readyDeploymentPod(t, c, envNamespace, oauthDeploymentName), int(defaults.OAuthProxyPortNumber))
	require.NoError(t, err)
	dexAddr, err := internal.PortForward(t.Context(), cfg, easyAuthNamespace, readyDeploymentPod(t, c, easyAuthNamespace, easyAuthReleaseName+"-dex"), issuerPort)
	require.NoError(t, err)

	return &easyAuthBrowser{
		webURL:    webURL,
		issuerURL: issuerURL,
		transport: &hostRewriteTransport{
			base:  http.DefaultTransport.(*http.Transport).Clone(),
			hosts: map[string]string{webURL.Host: proxyAddr, issuerURL.Host: dexAddr},
		},
	}
}

// newClient returns a client with an empty cookie jar that follows redirects
func (b *easyAuthBrowser) newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{Transport: b.transport, Jar: jar, Timeout: 30 * time.Second}
}

// withoutRedirects returns a copy of c, sharing its cookie jar, that returns redirects instead of following them
func withoutRedirects(c *http.Client) *http.Client {
	noRedirects := *c
	noRedirects.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &noRedirects
}

// openLoginForm opens the web component, which redirects to the Dex login form, and returns the URL of the form.
// Retries until the controller has registered the redirect URI of the web component in Dex.
func (b *easyAuthBrowser) openLoginForm(t *testing.T, c *http.Client) *url.URL {
	t.Helper()
	var loginURL *url.URL
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		resp, err := c.Get(b.webURL.String())
		if !assert.NoError(collect, err) {
			return
		}
		body := readBody(collect, resp)
		if assert.Equal(collect, http.StatusOK, resp.StatusCode, "body: %s", body) &&
			assert.Equal(collect, b.issuerURL.Host, resp.Request.URL.Host) &&
			assert.Contains(collect, body, `name="password"`) {
			loginURL = resp.Request.URL
		}
	}, 2*time.Minute, 2*time.Second, "%s should redirect to the Dex login form", b.webURL)
	return loginURL
}

// submitLogin posts the Dex login form, follows the redirects and returns the last response
func (b *easyAuthBrowser) submitLogin(t *testing.T, c *http.Client, loginURL *url.URL, username, password string) (*http.Response, string) {
	t.Helper()
	resp, err := c.PostForm(loginURL.String(), url.Values{"login": {username}, "password": {password}})
	require.NoError(t, err)
	return resp, readBody(t, resp)
}

func (b *easyAuthBrowser) get(t *testing.T, c *http.Client, u *url.URL) (*http.Response, string) {
	t.Helper()
	resp, err := c.Get(u.String())
	require.NoError(t, err)
	return resp, readBody(t, resp)
}

// assertRedirectedToDex asserts that the OAuth2 proxy redirects to the Dex authorization endpoint
func (b *easyAuthBrowser) assertRedirectedToDex(t *testing.T, resp *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	location, err := resp.Location()
	require.NoError(t, err)
	assert.Equal(t, b.issuerURL.Host, location.Host)
	assert.Equal(t, strings.TrimSuffix(b.issuerURL.Path, "/")+"/auth", location.Path)
	redirectURI := location.Query().Get("redirect_uri")
	assert.Equal(t, "https://"+b.webURL.Host+"/oauth2/callback", redirectURI)
}

func readBody(t require.TestingT, resp *http.Response) string {
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// hostRewriteTransport sends requests for the hosts in the map, as plain HTTP, to the mapped local address,
// and keeps the original URL and Host header
type hostRewriteTransport struct {
	base  http.RoundTripper
	hosts map[string]string
}

func (h *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	addr, ok := h.hosts[req.URL.Host]
	if !ok {
		return nil, fmt.Errorf("unexpected request to %s", req.URL)
	}
	forwarded := req.Clone(req.Context())
	forwarded.URL.Scheme = "http"
	forwarded.URL.Host = addr
	forwarded.Host = req.URL.Host
	resp, err := h.base.RoundTrip(forwarded)
	if resp != nil {
		resp.Request = req
	}
	return resp, err
}

// readyDeploymentPod returns the newest ready pod of a deployment, which is not being deleted
func readyDeploymentPod(t *testing.T, c client.Client, namespace, name string) string {
	t.Helper()
	var podName string
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		deployment := &appsv1.Deployment{}
		if !assert.NoError(collect, c.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: name}, deployment)) {
			return
		}
		pods := &corev1.PodList{}
		if !assert.NoError(collect, c.List(t.Context(), pods, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels))) {
			return
		}
		var newest *corev1.Pod
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.DeletionTimestamp != nil || !isPodReady(pod) {
				continue
			}
			if newest == nil || pod.CreationTimestamp.After(newest.CreationTimestamp.Time) {
				newest = pod
			}
		}
		if assert.NotNil(collect, newest, "deployment %s/%s should have a ready pod", namespace, name) {
			podName = newest.Name
		}
	}, time.Minute, time.Second)
	return podName
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// createEasyAuthTestApp creates the namespace and RadixRegistration required for a RadixApplication
// to be admitted by the radix-webhook, and removes them when the test finishes. Returns the AppID.
func createEasyAuthTestApp(t *testing.T, c client.Client, appName string) v1.ULID {
	t.Helper()
	ns := &corev1.Namespace{
		Name:   appName + "-app",
		Labels: map[string]string{kube.RadixAppLabel: appName},
	}
	require.NoError(t, c.Create(t.Context(), ns))
	t.Cleanup(func() { _ = c.Delete(context.Background(), ns) })

	rr := &v1.RadixRegistration{
		Name: appName,
		Spec: v1.RadixRegistrationSpec{
			AppID:             v1.ULID{ULID: ulid.Make()},
			CloneURL:          "git@github.com:equinor/" + appName + ".git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}
	require.NoError(t, c.Create(t.Context(), rr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), rr) })
	return rr.Spec.AppID
}
