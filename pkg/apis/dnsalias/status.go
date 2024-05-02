package dnsalias

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (s *syncer) restoreStatus(ctx context.Context) error {
	restoredStatus, ok := s.radixDNSAlias.Annotations[kube.RestoredStatusAnnotation]
	if !ok || len(restoredStatus) == 0 {
		return nil
	}
	if !reflect.ValueOf(s.radixDNSAlias.Status).IsZero() {
		return nil
	}
	var status radixv1.RadixDNSAliasStatus
	if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
		return fmt.Errorf("unable to deserialize status from annotation: %w", err)
	}
	return s.updateStatus(ctx, func(currStatus *radixv1.RadixDNSAliasStatus) {
		*currStatus = status
	})
}

func (s *syncer) syncStatus(ctx context.Context, syncErr error) error {
	syncCompleteTime := metav1.Now()
	err := s.updateStatus(ctx, func(currStatus *radixv1.RadixDNSAliasStatus) {
		currStatus.Reconciled = &syncCompleteTime
		if syncErr != nil {
			currStatus.Condition = radixv1.RadixDNSAliasFailed
			currStatus.Message = syncErr.Error()
			return
		}
		currStatus.Condition = radixv1.RadixDNSAliasSucceeded
		currStatus.Message = ""
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}
	return syncErr
}

func (s *syncer) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixDNSAliasStatus)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixDNSAlias, err := s.radixClient.RadixV1().RadixDNSAliases().Get(ctx, s.radixDNSAlias.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&radixDNSAlias.Status)
		updatedRadixDNSAlias, err := s.radixClient.
			RadixV1().
			RadixDNSAliases().
			UpdateStatus(ctx, radixDNSAlias, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.radixDNSAlias = updatedRadixDNSAlias
		return err
	})
}
