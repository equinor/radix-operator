package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	oauth2TestTimeout         = 5 * time.Minute
	oauth2TestStabilityPeriod = 30 * time.Second
)

// TestOAuth2SystemManagedRedis deploys an nginx component with OAuth2 and a system managed redis
// session store through the pipeline runner, using the radix-flux style oauth2SessionStoreTemplate
// overlay from the Helm values, and verifies that the component, redis and oauth2 proxy pods run
// without crash looping.
func TestOAuth2SystemManagedRedis(t *testing.T) {
	c := getClient(t)
	const (
		appName       = "oauth2-test"
		envName       = "dev"
		componentName = "web"
		jobName       = "deploy-dev"
	)

	readyCtx, cancelReady := context.WithTimeout(t.Context(), queueTestTimeout)
	defer cancelReady()
	require.NoError(t, WaitForDeploymentReady(readyCtx, c, "radix-system", "radix-operator"), "radix-operator deployment should be ready")

	appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	require.NoError(t, waitForSecret(t.Context(), c, appNamespace, defaults.GitPrivateKeySecretName, queueTestTimeout),
		"registration secret %s should be created in %s", defaults.GitPrivateKeySecretName, appNamespace)
	require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, queueTestTimeout), "pipeline RBAC should be provisioned in %s", appNamespace)

	// --- Pipeline runner: deploy job reads radixconfig.yaml from the git server and deploys it ---
	rj := &v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: appNamespace},
		Spec: v1.RadixJobSpec{
			AppName:      appName,
			PipeLineType: v1.Deploy,
			Deploy:       v1.RadixDeploySpec{ToEnvironment: envName},
			TriggeredBy:  "e2e",
		},
	}
	require.NoError(t, c.Create(t.Context(), rj), "should create deploy job")

	cond, err := waitForJobCondition(t.Context(), c, appNamespace, jobName, v1.RadixJobCondition.IsDone, oauth2TestTimeout)
	require.NoError(t, err, "deploy job should finish, last condition: %s", cond)
	if cond != v1.JobSucceeded {
		finished := &v1.RadixJob{}
		_ = c.Get(t.Context(), client.ObjectKeyFromObject(rj), finished)
		require.Failf(t, "deploy job did not succeed", "condition: %s, steps: %+v", cond, finished.Status.Steps)
	}

	ra := &v1.RadixApplication{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: appNamespace, Name: appName}, ra), "pipeline should apply the RadixApplication")
	require.Len(t, ra.Spec.Components, 1)
	assert.Equal(t, v1.SessionStoreSystemManaged, ra.Spec.Components[0].Authentication.OAuth2.SessionStoreType)

	envNamespace := utils.GetEnvironmentNamespace(appName, envName)
	var activeRD *v1.RadixDeployment
	require.NoError(t, wait.PollUntilContextTimeout(t.Context(), queueTestPollInterval, oauth2TestTimeout, true, func(ctx context.Context) (bool, error) {
		rds := &v1.RadixDeploymentList{}
		if err := c.List(ctx, rds, client.InNamespace(envNamespace)); err != nil {
			return false, nil
		}
		for i := range rds.Items {
			if rds.Items[i].Status.Condition == v1.DeploymentActive {
				activeRD = &rds.Items[i]
				return true, nil
			}
		}
		return false, nil
	}), "an active RadixDeployment should exist in %s", envNamespace)
	rdComponent := activeRD.GetComponentByName(componentName)
	require.NotNil(t, rdComponent, "RadixDeployment should contain component %s", componentName)
	assert.Equal(t, v1.SessionStoreSystemManaged, rdComponent.Authentication.OAuth2.SessionStoreType)
	assert.Equal(t, v1.AzureWorkloadIdentity, rdComponent.Authentication.OAuth2.Credentials)

	redisName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	proxyName := utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)

	proxySAName := rdComponent.Authentication.OAuth2.GetServiceAccountName(componentName)
	require.NoError(t, wait.PollUntilContextTimeout(t.Context(), queueTestPollInterval, oauth2TestTimeout, true, func(ctx context.Context) (bool, error) {
		sa := &corev1.ServiceAccount{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: envNamespace, Name: proxySAName}, sa); err != nil {
			return false, nil
		}
		return sa.Annotations["azure.workload.identity/client-id"] == rdComponent.Authentication.OAuth2.ClientID, nil
	}), "oauth2 proxy service account %s should be annotated with the workload identity client id", proxySAName)

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
