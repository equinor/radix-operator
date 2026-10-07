package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	oauth2TestTimeout         = 5 * time.Minute
	oauth2TestStabilityPeriod = 30 * time.Second
	oaurth2TestTimeout        = 60 * time.Second
	oaurth2TestPollInterval   = 100 * time.Millisecond
)

// TestOAuth2SystemManagedRedis deploys an nginx component with OAuth2 and a system managed redis
// session store through the pipeline runner, using the radix-flux style oauth2SessionStoreTemplate
// overlay from the Helm values, and verifies that the component, redis and oauth2 proxy pods run
// without crash looping.
func TestOAuth2SystemManagedRedis(t *testing.T) {
	t.Parallel()
	c := getClient(t)
	const (
		appName       = "oauth2-test"
		envName       = "dev"
		componentName = "web"
		jobName       = "deploy-dev"
	)

	readyCtx, cancelReady := context.WithTimeout(t.Context(), oaurth2TestTimeout)
	defer cancelReady()
	require.NoError(t, WaitForDeploymentReady(readyCtx, c, "radix-system", "radix-operator"), "radix-operator deployment should be ready")

	appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	require.NoError(t, waitForSecret(t.Context(), c, appNamespace, defaults.GitPrivateKeySecretName, oaurth2TestTimeout),
		"registration secret %s should be created in %s", defaults.GitPrivateKeySecretName, appNamespace)
	require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, oaurth2TestTimeout), "pipeline RBAC should be provisioned in %s", appNamespace)

	// --- Pipeline runner: deploy job reads radixconfig.yaml from the git server and deploys it ---
	rj := &v1.RadixJob{
		Name: jobName, Namespace: appNamespace,
		Spec: v1.RadixJobSpec{
			AppName:      appName,
			PipeLineType: v1.Deploy,
			Deploy:       v1.RadixDeploySpec{ToEnvironment: envName},
			TriggeredBy:  "e2e",
		},
	}
	require.NoError(t, c.Create(t.Context(), rj), "should create deploy job")

	cond, err := waitForJobCondition(t.Context(), c, appNamespace, jobName, v1.RadixJobCondition.IsDone, oauth2TestTimeout)
	if err != nil || cond != v1.JobSucceeded {
		t.Logf("deploy job failed or did not succeed, last condition: %s, error: %v", cond, err)
		logRadixJobDiagnostics(t, c, appNamespace, jobName)
		logRadixOperatorLogs(t, c, appName)
	}
	require.NoError(t, err, "deploy job should finish, last condition: %s", cond)
	require.Equal(t, v1.JobSucceeded, cond, "deploy job should succeed")

	ra := &v1.RadixApplication{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: appNamespace, Name: appName}, ra), "pipeline should apply the RadixApplication")
	require.Len(t, ra.Spec.Components, 1)
	assert.Equal(t, v1.SessionStoreSystemManaged, ra.Spec.Components[0].Authentication.OAuth2.SessionStoreType)

	envNamespace := utils.GetEnvironmentNamespace(appName, envName)
	var activeRD *v1.RadixDeployment
	require.NoError(t, wait.PollUntilContextTimeout(t.Context(), oaurth2TestPollInterval, oauth2TestTimeout, true, func(ctx context.Context) (bool, error) {
		rds := &v1.RadixDeploymentList{}
		if err := c.List(ctx, rds, client.InNamespace(envNamespace)); err != nil {
			return false, nil
		}
		for i := range rds.Items {
			if rds.Items[i].Status.Condition == v1.DeploymentActive && rds.Items[i].Status.ReconcileStatus == v1.RadixDeploymentReconcileSucceeded {
				activeRD = &rds.Items[i]
				return true, nil
			}
		}
		return false, nil
	}), "an active RadixDeployment should exist in %s", envNamespace)
	rdComponent := activeRD.GetComponentByName(componentName)
	require.NotNil(t, rdComponent, "RadixDeployment should contain component %s", componentName)
	require.Equal(t, v1.SessionStoreSystemManaged, rdComponent.Authentication.OAuth2.SessionStoreType)
	require.Equal(t, v1.Secret, rdComponent.Authentication.OAuth2.Credentials)

	oauthSecrets := &corev1.SecretList{}
	require.NoError(t, c.List(t.Context(), oauthSecrets, client.InNamespace(envNamespace), client.MatchingLabels{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthProxyAuxiliaryComponentType,
	}))
	require.Len(t, oauthSecrets.Items, 1, "one OAuth secret should exist in %s", envNamespace)
	oauthSecret := &oauthSecrets.Items[0]
	clientSecret := random.RandString(32)
	require.NotEmpty(t, clientSecret)
	oauthSecret.Data[defaults.OAuthClientSecretKeyName] = []byte(clientSecret)
	require.NoError(t, c.Update(t.Context(), oauthSecret), "should update OAuth client secret")

	redisName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	proxyName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)

	// Max restarts per deployment; the proxy may restart once while redis is starting.
	deployments := map[string]int32{componentName: 0, redisName: 0, proxyName: 1}
	for name := range deployments {
		require.NoError(t, wait.PollUntilContextTimeout(t.Context(), 2*time.Second, oauth2TestTimeout, true, func(ctx context.Context) (bool, error) {
			ok, _ := checkDeploymentPods(ctx, c, envNamespace, name, -1)
			return ok, nil
		}), "pods for %s should be running and ready: %s", name, describeDeploymentPods(t.Context(), c, envNamespace, name))
	}

	// Make sure the pods stay up and do not end up in CrashLoopBackOff.
	time.Sleep(oauth2TestStabilityPeriod)
	for name, maxRestarts := range deployments {
		ok, msg := checkDeploymentPods(t.Context(), c, envNamespace, name, maxRestarts)
		assert.True(t, ok, "pods for %s should be stable: %s", name, msg)
	}
}

// checkDeploymentPods reports whether all pods of a deployment are running and ready with at most
// maxRestarts container restarts (maxRestarts < 0 disables the restart check).
func checkDeploymentPods(ctx context.Context, c client.Client, namespace, name string, maxRestarts int32) (bool, string) {
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return false, err.Error()
	}
	if deployment.Spec.Selector == nil {
		return false, "deployment has no selector"
	}
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		return false, err.Error()
	}
	if len(pods.Items) == 0 {
		return false, "no pods found"
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false, fmt.Sprintf("pod %s is %s", pod.Name, pod.Status.Phase)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return false, fmt.Sprintf("container %s in pod %s is not ready", cs.Name, pod.Name)
			}
			if maxRestarts >= 0 && cs.RestartCount > maxRestarts {
				return false, fmt.Sprintf("container %s in pod %s restarted %d times (max %d)", cs.Name, pod.Name, cs.RestartCount, maxRestarts)
			}
		}
	}
	return true, ""
}

// describeDeploymentPods returns a short summary of the pod and container states of a deployment.
func describeDeploymentPods(ctx context.Context, c client.Client, namespace, name string) string {
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil || deployment.Spec.Selector == nil {
		return "deployment not found"
	}
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		return err.Error()
	}
	var parts []string
	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			state := "running"
			if cs.State.Waiting != nil {
				state = fmt.Sprintf("waiting (%s: %s)", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			} else if cs.State.Terminated != nil {
				state = fmt.Sprintf("terminated (%s)", cs.State.Terminated.Reason)
			}
			parts = append(parts, fmt.Sprintf("%s/%s: phase=%s ready=%t restarts=%d %s", pod.Name, cs.Name, pod.Status.Phase, cs.Ready, cs.RestartCount, state))
		}
	}
	return strings.Join(parts, "; ")
}
