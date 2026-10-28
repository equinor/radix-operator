package e2e

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const easyAuthAnnotation = "radix.equinor.com/easyauth"

// TestRadixEasyAuthMutatingWebhook verifies that the radix-easyauth mutating webhook only mutates
// RadixApplications that have the easyauth annotation
func TestRadixEasyAuthMutatingWebhook(t *testing.T) {
	t.Parallel()
	c := getClient(t)

	t.Run("sets testMutate label when RadixApplication has easyauth annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-annotated"
		createEasyAuthTestApp(t, c, appName)

		ra := &v1.RadixApplication{
			Name:        appName,
			Namespace:   appName + "-app",
			Annotations: map[string]string{easyAuthAnnotation: ""},
			Labels:      map[string]string{"existing": "label"},
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{{Name: "dev"}},
			},
		}
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		// Create updates ra with the object persisted by the API server, after mutation
		assert.Equal(t, appName, ra.Labels["testMutate"])
		assert.Equal(t, "label", ra.Labels["existing"], "existing labels should be preserved")
	})

	t.Run("does not set testMutate label when RadixApplication lacks easyauth annotation", func(t *testing.T) {
		t.Parallel()
		appName := "test-easyauth-not-annotated"
		createEasyAuthTestApp(t, c, appName)

		ra := &v1.RadixApplication{
			Name:      appName,
			Namespace: appName + "-app",
			Spec: v1.RadixApplicationSpec{
				Environments: []v1.Environment{{Name: "dev"}},
			},
		}
		require.NoError(t, c.Create(t.Context(), ra))
		t.Cleanup(func() { _ = c.Delete(context.Background(), ra) })

		assert.NotContains(t, ra.Labels, "testMutate")
	})
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
