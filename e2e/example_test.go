package e2e

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestExample demonstrates how to write e2e tests using the controller-runtime client
func TestExample(t *testing.T) {
	// Skip this test by default (it's just an example)
	t.Skip("This is an example test, not meant to be run")

	// Get the client from manager
	c := getClient(t)

	t.Run("create and delete RadixRegistration", func(t *testing.T) {
		// Define the RadixRegistration
		rr := &v1.RadixRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-example-app",
			},
			Spec: v1.RadixRegistrationSpec{
				CloneURL:          "git@github.com:equinor/my-test-app.git",
				AdGroups:          []string{"123"},
				ConfigBranch:      "main",
				ConfigurationItem: "12345",
			},
		}

		// Create the RadixRegistration
		err := c.Create(t.Context(), rr)
		require.NoError(t, err, "failed to create RadixRegistration")
		assert.Equal(t, "test-example-app", rr.Name)

		// Get the created resource to verify it exists
		retrieved := &v1.RadixRegistration{}
		err = c.Get(t.Context(), client.ObjectKey{Name: "test-example-app"}, retrieved)
		require.NoError(t, err, "RadixRegistration not found")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "git@github.com:equinor/my-test-app.git", retrieved.Spec.CloneURL)

		// Update the RadixRegistration
		retrieved.Spec.Owner = "example@equinor.com"
		err = c.Update(t.Context(), retrieved)
		require.NoError(t, err, "failed to update RadixRegistration")

		// Verify the update
		updated := &v1.RadixRegistration{}
		err = c.Get(t.Context(), client.ObjectKey{Name: "test-example-app"}, updated)
		require.NoError(t, err)
		assert.Equal(t, "example@equinor.com", updated.Spec.Owner)

		// Clean up
		err = c.Delete(t.Context(), retrieved)
		require.NoError(t, err, "failed to delete RadixRegistration")
	})

	t.Run("list RadixRegistrations", func(t *testing.T) {
		// List all RadixRegistrations
		var list v1.RadixRegistrationList
		err := c.List(t.Context(), &list)
		require.NoError(t, err, "failed to list RadixRegistrations")

		t.Logf("Found %d RadixRegistrations", len(list.Items))
	})
}
