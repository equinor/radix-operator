package e2e

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixBatchWebhookSmokeTest tests that the resource requirements validator is
// registered for RadixBatch admission.
func TestRadixBatchWebhookSmokeTest(t *testing.T) {
	c := getClient(t)

	newBatch := func(resources *v1.ResourceRequirements) *v1.RadixBatch {
		return &v1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-batch-validation",
				Namespace: "default",
			},
			Spec: v1.RadixBatchSpec{
				RadixDeploymentJobRef: v1.RadixDeploymentJobComponentSelector{
					LocalObjectReference: v1.LocalObjectReference{Name: "any-rd"},
					Job:                  "compute",
				},
				Jobs: []v1.RadixBatchJob{
					{Name: "job-1", Resources: resources},
				},
			},
		}
	}

	t.Run("accepts job with valid resource requirements", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "250m", "memory": "256Mi"},
			Limits:   v1.ResourceList{"cpu": "1", "memory": "1Gi"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.NoError(t, err, "Should accept job with valid resource requirements")
	})

	t.Run("accepts job without resource requirements", func(t *testing.T) {
		rb := newBatch(nil)

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.NoError(t, err, "Should accept job without resource requirements")
	})

	t.Run("accepts job with cpu at the maximum", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "1k"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.NoError(t, err, "Should accept cpu quantity at the maximum")
	})

	t.Run("rejects job when request exceeds limit", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "500m"},
			Limits:   v1.ResourceList{"cpu": "250m"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "exceeds limit")
	})

	t.Run("rejects job with unsupported resource type", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"storage": "1Gi"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "only cpu and memory are allowed")
	})

	t.Run("rejects job with a text quantity", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "one"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "invalid quantity")
	})

	t.Run("rejects job with negative quantity", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"memory": "-1Mi"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "zero or negative quantity")
	})

	t.Run("rejects job with zero quantity", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "0"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "zero or negative quantity")
	})

	t.Run("rejects job with cpu above the maximum", func(t *testing.T) {
		rb := newBatch(&v1.ResourceRequirements{
			Limits: v1.ResourceList{"cpu": "1001"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "is too high")
	})
}
