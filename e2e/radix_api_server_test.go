package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRadixApiServerIsRunning(t *testing.T) {
	c := getClient(t)

	t.Run("deployment is ready", func(t *testing.T) {
		err := WaitForDeploymentReady(t.Context(), c, "radix-system", "radix-api-server")
		require.NoError(t, err, "radix-api-server deployment should be ready")
	})

	t.Run("pod is running and containers are ready", func(t *testing.T) {
		pods := &corev1.PodList{}
		err := c.List(t.Context(), pods, client.InNamespace("radix-system"), client.MatchingLabels{"app.kubernetes.io/name": "radix-api-server"})
		require.NoError(t, err)
		require.NotEmpty(t, pods.Items, "expected at least one radix-api-server pod")

		pod := pods.Items[0]
		assert.Equal(t, corev1.PodRunning, pod.Status.Phase, "radix-api-server pod should be running")

		for _, cs := range pod.Status.ContainerStatuses {
			assert.True(t, cs.Ready, fmt.Sprintf("container %s should be ready", cs.Name))
			assert.Zero(t, cs.RestartCount, fmt.Sprintf("container %s should not have restarted", cs.Name))
		}
	})

	t.Run("service exists with expected ports", func(t *testing.T) {
		svc := &corev1.Service{}
		err := c.Get(t.Context(), client.ObjectKey{Namespace: "radix-system", Name: "radix-api-server"}, svc)
		require.NoError(t, err, "radix-api-server service should exist")

		var foundHTTPPort, foundMetricsPort bool
		for _, port := range svc.Spec.Ports {
			if port.Name == "http" && port.Port == 3002 {
				foundHTTPPort = true
			}
			if port.Name == "metrics" && port.Port == 9090 {
				foundMetricsPort = true
			}
		}
		assert.True(t, foundHTTPPort, "service should have http port 3002")
		assert.True(t, foundMetricsPort, "service should have metrics port 9090")
	})

	t.Run("unauthenticated API request returns 403", func(t *testing.T) {
		cfg := testManager.GetConfig()
		transport, err := rest.TransportFor(cfg)
		require.NoError(t, err)

		httpClient := &http.Client{Transport: transport}
		proxyURL := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%d/proxy/api/v1/applications",
			cfg.Host, "radix-system", "radix-api-server", 3002)

		resp, err := httpClient.Get(proxyURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "unauthenticated request should return 403, body: %s", string(body))
	})

	t.Run("health endpoint returns 200", func(t *testing.T) {
		cfg := testManager.GetConfig()
		transport, err := rest.TransportFor(cfg)
		require.NoError(t, err)

		httpClient := &http.Client{Transport: transport}
		proxyURL := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%d/proxy/health/",
			cfg.Host, "radix-system", "radix-api-server", 3002)

		resp, err := httpClient.Get(proxyURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "health endpoint should return 200")
	})
}
