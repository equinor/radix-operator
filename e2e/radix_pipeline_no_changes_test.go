package e2e

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestBuildDeployStopsWhenRadixConfigIsUnchanged verifies that a BuildDeploy pipeline for an already deployed commit
// stops with JobStoppedNoChanges when radixconfig.yaml is unchanged, even though the RadixApplication stored by the
// API server differs from radixconfig.yaml because of CRD defaults or radix-easyauth mutations. A genuine change to
// radixconfig.yaml must still be deployed.
// The radixconfig files are in testdata/gitserver/repos/equinor/<appName>.git.
func TestBuildDeployStopsWhenRadixConfigIsUnchanged(t *testing.T) {
	t.Parallel()
	c := getClient(t)

	tests := map[string]struct {
		appName string
		// assertAdmissionChangedSpec checks that the stored RadixApplication differs from radixconfig.yaml,
		// so the test fails if the scenario stops exercising admission changes
		assertAdmissionChangedSpec func(t *testing.T, ra *v1.RadixApplication, appID v1.ULID)
	}{
		"API server defaults": {
			appName: "build-deploy-defaults-test",
			assertAdmissionChangedSpec: func(t *testing.T, ra *v1.RadixApplication, _ v1.ULID) {
				require.NotNil(t, ra.Spec.Environments[0].Build.WebhookEnabled, "the API server should default webhookEnabled")
				assert.True(t, *ra.Spec.Environments[0].Build.WebhookEnabled)
			},
		},
		"radix-easyauth mutations": {
			appName: "build-deploy-easyauth-test",
			assertAdmissionChangedSpec: func(t *testing.T, ra *v1.RadixApplication, appID v1.ULID) {
				web := ra.Spec.Components[0]
				require.NotNil(t, web.Authentication)
				assert.Equal(t, expectedEasyAuthOAuth2(appID), web.Authentication.OAuth2, "radix-easyauth should replace the OAuth2 config")
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			const (
				envName     = "dev"
				waitTimeout = 60 * time.Second
			)
			appName := tt.appName
			envNamespace := utils.GetEnvironmentNamespace(appName, envName)

			appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
			rr := &v1.RadixRegistration{}
			require.NoError(t, testManager.GetAPIReader().Get(t.Context(), client.ObjectKey{Name: appName}, rr))
			require.NoError(t, waitForSecret(t.Context(), c, appNamespace, defaults.GitPrivateKeySecretName, waitTimeout),
				"registration secret %s should be created in %s", defaults.GitPrivateKeySecretName, appNamespace)
			require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, waitTimeout), "pipeline RBAC should be provisioned in %s", appNamespace)

			runBuildDeployJob(t, c, appName, "build-deploy-initial", envName, "", v1.JobSucceeded)
			initialRD := getSingleRadixDeploymentForJob(t, c, envNamespace, "build-deploy-initial")
			commitID := initialRD.Annotations[kube.RadixCommitAnnotation]
			require.NotEmpty(t, commitID, "RadixDeployment %s should have the commit annotation", initialRD.Name)

			ra := &v1.RadixApplication{}
			require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: appNamespace, Name: appName}, ra))
			tt.assertAdmissionChangedSpec(t, ra, rr.Spec.AppID)

			// The prepare pipeline step compares radixconfig.yaml with the active RadixDeployment only
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				rd := &v1.RadixDeployment{}
				if assert.NoError(collect, c.Get(t.Context(), client.ObjectKeyFromObject(initialRD), rd)) {
					assert.Equal(collect, v1.DeploymentActive, rd.Status.Condition)
				}
			}, waitTimeout, time.Second, "RadixDeployment %s should become active", initialRD.Name)

			runBuildDeployJob(t, c, appName, "build-deploy-unchanged", envName, commitID, v1.JobStoppedNoChanges)
			rds := &v1.RadixDeploymentList{}
			require.NoError(t, c.List(t.Context(), rds, client.InNamespace(envNamespace)))
			require.Len(t, rds.Items, 1, "an unchanged BuildDeploy pipeline should not create a RadixDeployment")

			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(rr), rr))
			original := rr.DeepCopy()
			rr.Spec.RadixConfigFullName = "radixconfig-changed.yaml"
			require.NoError(t, c.Patch(t.Context(), rr, client.MergeFrom(original)))

			runBuildDeployJob(t, c, appName, "build-deploy-changed", envName, commitID, v1.JobSucceeded)
			changedRD := getSingleRadixDeploymentForJob(t, c, envNamespace, "build-deploy-changed")
			assert.Equal(t, commitID, changedRD.Annotations[kube.RadixCommitAnnotation])
			web := changedRD.GetComponentByName("web")
			require.NotNil(t, web, "RadixDeployment %s should contain component web", changedRD.Name)
			assert.Equal(t, "true", web.GetEnvironmentVariables()["E2E_CONFIG_CHANGED"], "RadixDeployment %s should contain the changed config", changedRD.Name)
		})
	}
}

// runBuildDeployJob runs a BuildDeploy pipeline job for the branch envName, optionally for commitID,
// and requires that the job ends with expectedCondition
func runBuildDeployJob(t *testing.T, c client.Client, appName, jobName, envName, commitID string, expectedCondition v1.RadixJobCondition) {
	t.Helper()
	const timeout = 5 * time.Minute
	appNamespace := appName + "-app"
	rj := &v1.RadixJob{
		Name: jobName, Namespace: appNamespace,
		Spec: v1.RadixJobSpec{
			AppName:      appName,
			PipeLineType: v1.BuildDeploy,
			Build:        v1.RadixBuildSpec{Branch: envName, GitRef: envName, GitRefType: v1.GitRefBranch, CommitID: commitID}, //nolint:staticcheck
			TriggeredBy:  "e2e",
		},
	}
	require.NoError(t, c.Create(t.Context(), rj), "should create job %s", jobName)
	cond, err := waitForJobCondition(t.Context(), c, appNamespace, jobName, v1.RadixJobCondition.IsDone, timeout)
	if err != nil || cond != expectedCondition {
		logRadixJobDiagnostics(t, c, appNamespace, jobName)
		logRadixOperatorLogs(t, c, appName)
	}
	require.NoError(t, err, "job %s should finish, last condition: %s", jobName, cond)
	require.Equal(t, expectedCondition, cond, "job %s should end with condition %s", jobName, expectedCondition)
}

func getSingleRadixDeploymentForJob(t *testing.T, c client.Client, envNamespace, jobName string) *v1.RadixDeployment {
	t.Helper()
	rds := &v1.RadixDeploymentList{}
	require.NoError(t, c.List(t.Context(), rds, client.InNamespace(envNamespace), client.MatchingLabels{kube.RadixJobNameLabel: jobName}))
	require.Len(t, rds.Items, 1, "job %s should create one RadixDeployment", jobName)
	return &rds.Items[0]
}
