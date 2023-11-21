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

func (s *syncer) restoreStatus() error {
	restoredStatus, ok := s.radixDNSAlias.Annotations[kube.RestoredStatusAnnotation]
	if !ok || len(restoredStatus) == 0 {
		return nil
	}
	if !reflect.ValueOf(s.radixDNSAlias.Status).IsZero() {
		return nil
	}
	var status radixv1.RadixDNSAliasStatus
	if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
		return fmt.Errorf("unable to deserealise status from annotation: %w", err)
	}
	return s.updateStatus(func(currStatus *radixv1.RadixDNSAliasStatus) {
		*currStatus = status
	})
}

func (s *syncer) syncStatus() error {
	// syncCompleteTime := metav1.Now()
	// err := s.updateStatus(func(currStatus *radixv1.RadixDNSAliasStatus) {
	// 	// currStatus.Reconciled = &syncCompleteTime
	// })
	// if err != nil {
	// 	return fmt.Errorf("failed to sync status: %v", err)
	// }
	return nil
}

func (s *syncer) updateStatus(changeStatusFunc func(currStatus *radixv1.RadixDNSAliasStatus)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixDNSAlias, err := s.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), s.radixDNSAlias.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&radixDNSAlias.Status)
		updatedRadixDNSAlias, err := s.radixClient.
			RadixV1().
			RadixDNSAliases().
			UpdateStatus(context.TODO(), radixDNSAlias, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.radixDNSAlias = updatedRadixDNSAlias
		return err
	})
}
