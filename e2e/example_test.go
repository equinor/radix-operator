package e2e

import (
	"context"
	"testing"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExample demonstrates how to write e2e tests using the test helpers
func TestExample(t *testing.T) {
	// Skip this test by default (it's just an example)
	t.Skip("This is an example test, not meant to be run")

	// Get the test context
	ctx := getTestContext(t)

	// Get the kubernetes configuration
	config := getKubeConfig(t)

	// Create clients
	clients, err := NewClients(config)
	require.NoError(t, err, "failed to create clients")

	// Create test helpers
	helpers := NewTestHelpers(clients)

	t.Run("create and delete RadixRegistration", func(t *testing.T) {
		// Define the RadixRegistration spec
		spec := v1.RadixRegistrationSpec{
			CloneURL:     "git@github.com:equinor/my-test-app.git",
			AdGroups:     []string{"123"},
			ConfigBranch: "main",
		}

		// Create a timeout context for this operation
		createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Create the RadixRegistration
		rr, err := helpers.CreateRadixRegistration(createCtx, "test-example-app", spec)
		require.NoError(t, err, "failed to create RadixRegistration")
		assert.Equal(t, "test-example-app", rr.Name)

		// Wait for the resource to be available
		rr, err = helpers.WaitForRadixRegistration(createCtx, "test-example-app", 30*time.Second)
		require.NoError(t, err, "RadixRegistration not found")
		assert.NotNil(t, rr)

		// Update the RadixRegistration
		rr.Spec.Owner = "example@equinor.com"
		updated, err := helpers.UpdateRadixRegistration(createCtx, rr)
		require.NoError(t, err, "failed to update RadixRegistration")
		assert.Equal(t, "example@equinor.com", updated.Spec.Owner)

		// Clean up
		err = helpers.CleanupRadixRegistration(createCtx, "test-example-app")
		require.NoError(t, err, "failed to cleanup RadixRegistration")
	})

	t.Run("list RadixRegistrations", func(t *testing.T) {
		listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// List all RadixRegistrations
		list, err := helpers.ListRadixRegistrations(listCtx)
		require.NoError(t, err, "failed to list RadixRegistrations")

		t.Logf("Found %d RadixRegistrations", len(list.Items))
	})
}
