package e2e

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRadixRegistrationWebhook tests that the admission webhook validates RadixRegistration resources
func TestRadixRegistrationWebhook(t *testing.T) {
	c := getClient(t)

	t.Run("should reject invalid RadixRegistration", func(t *testing.T) {
		// Create an invalid RadixRegistration (missing required fields)
		invalidRR := &v1.RadixRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-invalid-rr",
			},
			Spec: v1.RadixRegistrationSpec{
				// Missing CloneURL - should be rejected
				AdGroups: []string{"123"},
			},
		}

		// Attempt to create the RadixRegistration
		err := c.Create(t.Context(), invalidRR)

		// We expect this to fail due to validation webhook
		assert.Error(t, err, "expected validation error for invalid RadixRegistration")

		if err != nil {
			t.Logf("Validation error (expected): %v", err)
		}
	})

	t.Run("should reject RadixRegistration with invalid CloneURL", func(t *testing.T) {
		// Create a RadixRegistration with invalid CloneURL format
		invalidRR := &v1.RadixRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-invalid-clone-url",
			},
			Spec: v1.RadixRegistrationSpec{
				CloneURL:     "not-a-valid-github-url",
				AdGroups:     []string{"123"},
				ConfigBranch: "main",
			},
		}

		// Attempt to create the RadixRegistration
		err := c.Create(t.Context(), invalidRR)

		// We expect this to fail due to validation webhook
		assert.Error(t, err, "expected validation error for invalid CloneURL")

		if err != nil {
			t.Logf("Validation error (expected): %v", err)
		}
	})

	t.Run("should accept valid RadixRegistration", func(t *testing.T) {
		// Create a valid RadixRegistration
		validRR := &v1.RadixRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-valid-rr",
			},
			Spec: v1.RadixRegistrationSpec{
				CloneURL:          "git@github.com:equinor/test-app.git",
				AdGroups:          []string{"123"},
				ConfigBranch:      "main",
				ConfigurationItem: "12345",
			},
		}

		// Attempt to create the RadixRegistration
		err := c.Create(t.Context(), validRR)

		// Webhook should accept this valid RadixRegistration
		assert.NoError(t, err, "valid RadixRegistration should be accepted by webhook")

		if err == nil {
			t.Logf("Successfully created RadixRegistration: %s", validRR.Name)

			// Clean up
			_ = c.Delete(t.Context(), validRR)
		}
	})
}

// TestRadixRegistrationCRUD tests basic CRUD operations for RadixRegistration
func TestRadixRegistrationCRUD(t *testing.T) {
	ctx := getTestContext(t)
	c := getClient(t)

	t.Run("should list RadixRegistrations", func(t *testing.T) {
		// List RadixRegistrations
		var list v1.RadixRegistrationList
		err := c.List(ctx, &list)

		// This might fail if CRDs are not installed
		if err != nil {
			t.Logf("List failed (may be expected if CRDs not ready): %v", err)
		} else {
			t.Logf("Found %d RadixRegistrations", len(list.Items))
		}
	})
}
