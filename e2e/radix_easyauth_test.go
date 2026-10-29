package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	easyAuthAnnotation = "radix.equinor.com/easyauth"
	// Must match config.oauth2.issuerUrl in testdata/radix-easyauth-values.yaml
	easyAuthOAuth2IssuerURL = "https://simpleauth.dev.radix.equinor.com"
)

// TestRadixEasyAuthMutatingWebhook verifies that the radix-easyauth mutating webhook only mutates
// RadixApplications that have the easyauth annotation
func TestRadixEasyAuthMutatingWebhook(t *testing.T) {
	t.Parallel()
	c := getClient(t)

	t.Run("overrides OAuth2 config on public components when RadixApplication has easyauth annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-oauth2"
		createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, map[string]string{easyAuthAnnotation: ""})
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		// Create updates ra with the object persisted by the API server, after mutation
		assertEasyAuthOAuth2(t, ra)
	})

	t.Run("overrides user changes to OAuth2 config on update", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-oauth2-update"
		createEasyAuthTestApp(t, c, appName)

		ra := newEasyAuthTestRA(appName, map[string]string{easyAuthAnnotation: ""})
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		web := &ra.Spec.Components[0]
		web.Authentication.OAuth2.ClientID = "user-changed"
		web.Authentication.OAuth2.SkipAuthRoutes = []string{"^/healthz$"}
		web.Authentication.OAuth2.OIDC.IssuerURL = "https://login.example.com"
		web.EnvironmentConfig[0].Authentication = &v1.Authentication{
			OAuth2: &v1.OAuth2{ClientID: "env-changed"},
		}
		require.NoError(t, c.Update(t.Context(), ra))

		assertEasyAuthOAuth2(t, ra)
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
func assertEasyAuthOAuth2(t *testing.T, ra *v1.RadixApplication) {
	t.Helper()
	expected := expectedEasyAuthOAuth2(ra.Name)
	web, api := ra.Spec.Components[0], ra.Spec.Components[1]
	require.NotNil(t, web.Authentication)
	assert.Equal(t, expected, web.Authentication.OAuth2)
	require.Len(t, web.EnvironmentConfig, 1)
	if web.EnvironmentConfig[0].Authentication != nil {
		assert.Nil(t, web.EnvironmentConfig[0].Authentication.OAuth2, "environment OAuth2 config should be removed")
	}
	assert.Nil(t, api.Authentication, "private components should not be changed")
}

// expectedEasyAuthOAuth2 returns the OAuth2 config radix-easyauth sets on public components
func expectedEasyAuthOAuth2(appName string) *v1.OAuth2 {
	return &v1.OAuth2{
		ClientID:               appName + ".dev.radix.equinor.com",
		Credentials:            v1.Secret,
		Scope:                  "openid profile email offline_access",
		SetAuthorizationHeader: new(false),
		SetXAuthRequestHeaders: new(false),
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
		assertEasyAuthOAuth2(t, ra)

		rds := &v1.RadixDeploymentList{}
		require.NoError(t, c.List(t.Context(), rds,
			client.InNamespace(utils.GetEnvironmentNamespace(appName, envName)),
			client.MatchingLabels{kube.RadixJobNameLabel: rj.Name}))
		require.Len(t, rds.Items, 1, "job %s should create one RadixDeployment", rj.Name)
		rd := &rds.Items[0]

		web := rd.GetComponentByName("web")
		require.NotNil(t, web, "RadixDeployment %s should contain component web", rd.Name)
		require.NotNil(t, web.Authentication, "RadixDeployment %s component web should have authentication", rd.Name)
		assert.Equal(t, expectedEasyAuthOAuth2(appName), web.Authentication.OAuth2, "RadixDeployment %s component web should have the radix-easyauth OAuth2 config", rd.Name)

		api := rd.GetComponentByName("api")
		require.NotNil(t, api, "RadixDeployment %s should contain component api", rd.Name)
		assert.Nil(t, api.Authentication, "RadixDeployment %s component api should not have authentication", rd.Name)
	}
}

// createEasyAuthTestApp creates the namespace and RadixRegistration required for a RadixApplication
// to be admitted by the radix-webhook, and removes them when the test finishes
func createEasyAuthTestApp(t *testing.T, c client.Client, appName string) {
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
			CloneURL:          "git@github.com:equinor/" + appName + ".git",
			AdGroups:          []string{"test-group"},
			ConfigBranch:      "main",
			ConfigurationItem: "test-item",
		},
	}
	require.NoError(t, c.Create(t.Context(), rr))
	t.Cleanup(func() { _ = c.Delete(context.Background(), rr) })
}
