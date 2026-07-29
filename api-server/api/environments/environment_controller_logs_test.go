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

func auxPodLogURL(appName, envName, componentName, auxType, podName, rawQuery string) string {
	url := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/replicas/%s/logs",
		appName, envName, componentName, auxType, podName)
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	return url
}

// setupAuxLogTest registers the app, creates a matching pod, installs a pods/log reactor
// that captures PodLogOptions, and returns a pointer-to-pointer so the caller can read
// the captured value after the request completes.
func setupAuxLogTest(t *testing.T, commonTestUtils *commontest.Utils, kubeclient *kubefake.Clientset) **corev1.PodLogOptions {
	t.Helper()
	_, err := commonTestUtils.ApplyRegistration(operatorutils.NewRegistrationBuilder().WithName(anyAppName))
	require.NoError(t, err)

	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err = kubeclient.CoreV1().Pods(envNs).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      anyAuxPod,
			Namespace: envNs,
			Labels: map[string]string{
				kube.RadixAppLabel:                    anyAppName,
				kube.RadixAuxiliaryComponentLabel:     anyComponentName,
				kube.RadixAuxiliaryComponentTypeLabel: anyAuxType,
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	var captured *corev1.PodLogOptions
	kubeclient.PrependReactor("get", "pods/log", func(action testing2.Action) (bool, runtime.Object, error) {
		genericAction, ok := action.(testing2.GenericAction)
		require.True(t, ok)
		captured, ok = genericAction.GetValue().(*corev1.PodLogOptions)
		require.True(t, ok)
		return true, &runtime.Unknown{Raw: []byte("fake log content")}, nil
	})
	return &captured
}

// TestGetOAuthAuxiliaryResourcePodLog_LogOptions verifies that query parameters are
// correctly forwarded as PodLogOptions fields.
func TestGetOAuthAuxiliaryResourcePodLog_LogOptions(t *testing.T) {
	tests := []struct {
		query        string
		wantPrevious bool
		wantFollow   bool
	}{
		{query: "previous=true", wantPrevious: true},
		{query: "previous=false", wantPrevious: false},
		{query: "", wantPrevious: false},
		{query: "previous=true&follow=false", wantPrevious: true, wantFollow: false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			commonTestUtils, envUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
			capturedPtr := setupAuxLogTest(t, commonTestUtils, kubeclient)

			response := <-envUtils.ExecuteRequest("GET",
				auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, anyAuxPod, tt.query))

			assert.Equal(t, http.StatusOK, response.Code)
			require.NotNil(t, *capturedPtr, "expected GetLogs to have been called")
			assert.Equal(t, tt.wantPrevious, (*capturedPtr).Previous)
			assert.Equal(t, tt.wantFollow, (*capturedPtr).Follow)
		})
	}
}

// TestGetOAuthAuxiliaryResourcePodLog_Errors verifies error responses for bad input.
func TestGetOAuthAuxiliaryResourcePodLog_Errors(t *testing.T) {
	tests := []struct {
		name       string
		podName    string
		query      string
		wantStatus int
	}{
		{name: "invalid previous param", podName: anyAuxPod, query: "previous=notabool", wantStatus: http.StatusBadRequest},
		{name: "pod not found", podName: "nonexistent-pod", query: "", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commonTestUtils, envUtils, _, kubeclient, _, _, _, _, _ := setupTest(t, nil)
			_, err := commonTestUtils.ApplyRegistration(operatorutils.NewRegistrationBuilder().WithName(anyAppName))
			require.NoError(t, err)
			if tt.podName == anyAuxPod {
				envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
				_, err = kubeclient.CoreV1().Pods(envNs).Create(context.Background(), &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      anyAuxPod,
						Namespace: envNs,
						Labels: map[string]string{
							kube.RadixAppLabel:                    anyAppName,
							kube.RadixAuxiliaryComponentLabel:     anyComponentName,
							kube.RadixAuxiliaryComponentTypeLabel: anyAuxType,
						},
					},
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			response := <-envUtils.ExecuteRequest("GET",
				auxPodLogURL(anyAppName, anyEnvironment, anyComponentName, anyAuxType, tt.podName, tt.query))

			assert.Equal(t, tt.wantStatus, response.Code)
		})
	}
}
