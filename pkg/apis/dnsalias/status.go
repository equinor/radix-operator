package dnsalias

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (s *syncer) syncStatus(ctx context.Context, reconcileErr error) error {
	err := s.updateStatus(ctx, func(currStatus *radixv1.RadixDNSAliasStatus) {
		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = s.radixDNSAlias.Generation

		if reconcileErr != nil {
			currStatus.ReconcileStatus = radixv1.RadixDNSAliasReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = radixv1.RadixDNSAliasReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}
	return reconcileErr
}

func (s *syncer) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixDNSAliasStatus)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixDNSAlias, err := s.radixClient.RadixV1().RadixDNSAliases().Get(ctx, s.radixDNSAlias.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&radixDNSAlias.Status)
		updatedRadixDNSAlias, err := s.radixClient.RadixV1().RadixDNSAliases().UpdateStatus(ctx, radixDNSAlias, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.radixDNSAlias = updatedRadixDNSAlias
		return err
	})

}
