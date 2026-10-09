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
	t.Parallel()
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
		t.Parallel()
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "250m", "memory": "256Mi"},
			Limits:   v1.ResourceList{"cpu": "1", "memory": "1Gi"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.NoError(t, err, "Should accept job with valid resource requirements")
	})

	t.Run("rejects job with a text quantity", func(t *testing.T) {
		t.Parallel()
		rb := newBatch(&v1.ResourceRequirements{
			Requests: v1.ResourceList{"cpu": "one"},
		})

		err := c.Create(t.Context(), rb, client.DryRunAll)
		assert.ErrorContains(t, err, "invalid quantity")
	})
}
