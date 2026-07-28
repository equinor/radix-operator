package environments

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
)

const (
	anyAuxType = "oauth"
	anyAuxPod  = "oauth-pod-1"
)

// registerApp creates a minimal RadixRegistration so the authorization middleware
// does not reject the request with 404.
func registerApp(t *testing.T, commonTestUtils *commontest.Utils, appName string) {
	t.Helper()
	_, err := commonTestUtils.ApplyRegistration(operatorutils.NewRegistrationBuilder().WithName(appName))
	require.NoError(t, err)
}

// createAuxResourcePod creates a pod in the fake Kubernetes client with labels matching
// ForAuxiliaryResource(appName, componentName, auxType).
func createAuxResourcePod(t *testing.T, kubeclient *kubefake.Clientset, appName, envName, componentName, auxType, podName string) {
	t.Helper()
	envNs := operatorutils.GetEnvironmentNamespace(appName, envName)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: envNs,
			Labels: map[string]string{
				kube.RadixAppLabel:                    appName,
				kube.RadixAuxiliaryComponentLabel:     componentName,
				kube.RadixAuxiliaryComponentTypeLabel: auxType,
			},
		},
	}
	_, err := kubeclient.CoreV1().Pods(envNs).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
}

// auxPodLogURL returns the URL for the GetOAuthAuxiliaryResourcePodLog route.
func auxPodLogURL(appName, envName, componentName, auxType, podName, rawQuery string) string {
	url := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/replicas/%s/logs",
		appName, envName, componentName, auxType, podName)
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	return url
}

// captureLogOptsReactor returns a PrependReactor function that captures the PodLogOptions
// passed to the fake Kubernetes GetLogs call and injects a dummy log body so the stream
// succeeds. The captured options are stored in the pointer returned.
func captureLogOptsReactor(t *testing.T, captured **corev1.PodLogOptions) testing2.ReactionFunc {
	t.Helper()
	return func(action testing2.Action) (bool, runtime.Object, error) {
		genericAction, ok := action.(testing2.GenericAction)
		require.True(t, ok, "expected testing2.GenericAction for pods/log get")
		opts, ok := genericAction.GetValue().(*corev1.PodLogOptions)
		require.True(t, ok, "expected *corev1.PodLogOptions as action value")
		*captured = opts
		return true, &runtime.Unknown{Raw: []byte("fake log content")}, nil
	}
}

// TestGetOAuthAuxiliaryResourcePodLog_PreviousTrue tests that ?previous=true is
// propagated as PodLogOptions.Previous=true when streaming aux-resource pod logs.
func TestGetOAuthAuxiliaryResourcePodLog_PreviousTrue_PassedToK8sLogOptions(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	createAuxResourcePod(t, kubeclient, anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod)

	var capturedOpts *corev1.PodLogOptions
	kubeclient.PrependReactor("get", "pods/log", captureLogOptsReactor(t, &capturedOpts))

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, "previous=true"))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, capturedOpts, "expected GetLogs to have been called")
	assert.True(t, capturedOpts.Previous, "expected PodLogOptions.Previous=true")
}

// TestGetOAuthAuxiliaryResourcePodLog_PreviousFalse tests that ?previous=false is
// propagated as PodLogOptions.Previous=false.
func TestGetOAuthAuxiliaryResourcePodLog_PreviousFalse_PassedToK8sLogOptions(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	createAuxResourcePod(t, kubeclient, anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod)

	var capturedOpts *corev1.PodLogOptions
	kubeclient.PrependReactor("get", "pods/log", captureLogOptsReactor(t, &capturedOpts))

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, "previous=false"))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, capturedOpts, "expected GetLogs to have been called")
	assert.False(t, capturedOpts.Previous, "expected PodLogOptions.Previous=false")
}

// TestGetOAuthAuxiliaryResourcePodLog_NoPreviousParam tests that omitting the previous
// query parameter results in PodLogOptions.Previous=false (default).
func TestGetOAuthAuxiliaryResourcePodLog_NoPreviousParam_DefaultsFalse(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	createAuxResourcePod(t, kubeclient, anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod)

	var capturedOpts *corev1.PodLogOptions
	kubeclient.PrependReactor("get", "pods/log", captureLogOptsReactor(t, &capturedOpts))

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, ""))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, capturedOpts, "expected GetLogs to have been called")
	assert.False(t, capturedOpts.Previous, "expected PodLogOptions.Previous=false when parameter is absent")
}

// TestGetOAuthAuxiliaryResourcePodLog_InvalidPreviousParam tests that an unparseable
// value for the previous query parameter causes the handler to return an error response.
func TestGetOAuthAuxiliaryResourcePodLog_InvalidPreviousParam_ReturnsError(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	createAuxResourcePod(t, kubeclient, anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod)

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, "previous=notabool"))
	response := <-responseChannel

	assert.NotEqual(t, http.StatusOK, response.Code, "expected non-200 response for invalid previous param")
}

// TestGetOAuthAuxiliaryResourcePodLog_PodNotFound tests that requesting logs for a pod
// that does not exist returns 404 Not Found.
func TestGetOAuthAuxiliaryResourcePodLog_PodNotFound_Returns404(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	// no pod is created – the List call will return an empty list

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, "nonexistent-pod", "previous=true"))
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
}

// TestGetOAuthAuxiliaryResourcePodLog_FollowAndPrevious tests that both follow and
// previous query parameters are forwarded independently.
func TestGetOAuthAuxiliaryResourcePodLog_FollowAndPrevious_BothPropagated(t *testing.T) {
	commonTestUtils, envControllerTestUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
	registerApp(t, commonTestUtils, anyAppName)
	createAuxResourcePod(t, kubeclient, anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod)

	var capturedOpts *corev1.PodLogOptions
	kubeclient.PrependReactor("get", "pods/log", captureLogOptsReactor(t, &capturedOpts))

	responseChannel := envControllerTestUtils.ExecuteRequest("GET",
		auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, "previous=true&follow=false"))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, capturedOpts)
	assert.True(t, capturedOpts.Previous)
	assert.False(t, capturedOpts.Follow)
}
