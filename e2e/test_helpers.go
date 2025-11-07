package e2e

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// TestHelpers provides utility functions for e2e tests
type TestHelpers struct {
	RadixClient radixclient.Interface
}

// NewTestHelpers creates a new TestHelpers instance
func NewTestHelpers(clients *Clients) *TestHelpers {
	return &TestHelpers{
		RadixClient: clients.RadixClient,
	}
}

// WaitForRadixRegistration waits for a RadixRegistration to exist
func (h *TestHelpers) WaitForRadixRegistration(ctx context.Context, name string, timeout time.Duration) (*v1.RadixRegistration, error) {
	var rr *v1.RadixRegistration

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		var err error
		rr, err = h.RadixClient.RadixV1().RadixRegistrations().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("timeout waiting for RadixRegistration %s: %w", name, err)
	}

	return rr, nil
}

// WaitForRadixRegistrationDeleted waits for a RadixRegistration to be deleted
func (h *TestHelpers) WaitForRadixRegistrationDeleted(ctx context.Context, name string, timeout time.Duration) error {
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := h.RadixClient.RadixV1().RadixRegistrations().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// If we get an error (not found), the resource is deleted
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for RadixRegistration %s to be deleted: %w", name, err)
	}

	return nil
}

// CleanupRadixRegistration deletes a RadixRegistration and waits for it to be removed
func (h *TestHelpers) CleanupRadixRegistration(ctx context.Context, name string) error {
	// Delete the resource
	err := h.RadixClient.RadixV1().RadixRegistrations().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete RadixRegistration %s: %w", name, err)
	}

	// Wait for it to be deleted
	return h.WaitForRadixRegistrationDeleted(ctx, name, 30*time.Second)
}

// CreateRadixRegistration creates a RadixRegistration with the given spec
func (h *TestHelpers) CreateRadixRegistration(ctx context.Context, name string, spec v1.RadixRegistrationSpec) (*v1.RadixRegistration, error) {
	rr := &v1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	created, err := h.RadixClient.RadixV1().RadixRegistrations().Create(ctx, rr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create RadixRegistration %s: %w", name, err)
	}

	return created, nil
}

// ListRadixRegistrations lists all RadixRegistrations
func (h *TestHelpers) ListRadixRegistrations(ctx context.Context) (*v1.RadixRegistrationList, error) {
	list, err := h.RadixClient.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list RadixRegistrations: %w", err)
	}
	return list, nil
}

// GetRadixRegistration gets a RadixRegistration by name
func (h *TestHelpers) GetRadixRegistration(ctx context.Context, name string) (*v1.RadixRegistration, error) {
	rr, err := h.RadixClient.RadixV1().RadixRegistrations().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get RadixRegistration %s: %w", name, err)
	}
	return rr, nil
}

// UpdateRadixRegistration updates a RadixRegistration
func (h *TestHelpers) UpdateRadixRegistration(ctx context.Context, rr *v1.RadixRegistration) (*v1.RadixRegistration, error) {
	updated, err := h.RadixClient.RadixV1().RadixRegistrations().Update(ctx, rr, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update RadixRegistration %s: %w", rr.Name, err)
	}
	return updated, nil
}
