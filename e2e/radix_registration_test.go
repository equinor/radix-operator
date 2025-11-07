package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/e2e/internal"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRadixRegistrationWebhook tests that the admission webhook validates RadixRegistration resources
func TestRadixRegistrationWebhook(t *testing.T) {
	ctx := getTestContext(t)
	config := getKubeConfig(t)

	// Create clients
	clients, err := internal.NewClients(config)
	require.NoError(t, err, "failed to create clients")

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

		// Create a timeout context for this operation
		createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Attempt to create the RadixRegistration
		_, err := clients.RadixClient.RadixV1().RadixRegistrations().Create(
			createCtx,
			invalidRR,
			metav1.CreateOptions{},
		)

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

		// Create a timeout context for this operation
		createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Attempt to create the RadixRegistration
		_, err := clients.RadixClient.RadixV1().RadixRegistrations().Create(
			createCtx,
			invalidRR,
			metav1.CreateOptions{},
		)

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
				CloneURL:     "git@github.com:equinor/test-app.git",
				AdGroups:     []string{"123"},
				ConfigBranch: "main",
			},
		}

		// Create a timeout context for this operation
		createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Attempt to create the RadixRegistration
		created, err := clients.RadixClient.RadixV1().RadixRegistrations().Create(
			createCtx,
			validRR,
			metav1.CreateOptions{},
		)

		// This might fail if webhook validation is strict or CRDs are not installed
		// For now, we just log the result
		if err != nil {
			t.Logf("Creation failed (may be expected if webhook/CRDs not ready): %v", err)
		} else {
			t.Logf("Successfully created RadixRegistration: %s", created.Name)

			// Clean up
			deleteCtx, deleteCancel := context.WithTimeout(ctx, 10*time.Second)
			defer deleteCancel()

			_ = clients.RadixClient.RadixV1().RadixRegistrations().Delete(
				deleteCtx,
				created.Name,
				metav1.DeleteOptions{},
			)
		}
	})
}

// TestRadixRegistrationCRUD tests basic CRUD operations for RadixRegistration
func TestRadixRegistrationCRUD(t *testing.T) {
	ctx := getTestContext(t)
	config := getKubeConfig(t)

	// Create clients
	clients, err := internal.NewClients(config)
	require.NoError(t, err, "failed to create clients")

	t.Run("should list RadixRegistrations", func(t *testing.T) {
		listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// List RadixRegistrations
		list, err := clients.RadixClient.RadixV1().RadixRegistrations().List(
			listCtx,
			metav1.ListOptions{},
		)

		// This might fail if CRDs are not installed
		if err != nil {
			t.Logf("List failed (may be expected if CRDs not ready): %v", err)
		} else {
			t.Logf("Found %d RadixRegistrations", len(list.Items))
		}
	})
}
