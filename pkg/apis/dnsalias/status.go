package dnsalias

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *syncer) updateStatus(changeStatusFunc func(currStatus *radixv1.RadixDNSAliasStatus)) error {
	changeStatusFunc(&s.radixDNSAlias.Status)
	updatedRadixDNSAlias, err := s.radixClient.
		RadixV1().
		RadixDNSAliases().
		UpdateStatus(context.TODO(), s.radixDNSAlias, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.radixDNSAlias = updatedRadixDNSAlias
	return nil
}

func (s *syncer) restoreStatus() error {
	if restoredStatus, ok := s.radixDNSAlias.Annotations[kube.RestoredStatusAnnotation]; ok && len(restoredStatus) > 0 {
		if reflect.ValueOf(s.radixDNSAlias.Status).IsZero() {
			var status radixv1.RadixDNSAliasStatus

			if err := json.Unmarshal([]byte(restoredStatus), &status); err != nil {
				return fmt.Errorf("unable to restore status for Radix DNS alias %s from annotation: %w", s.radixDNSAlias.GetName(), err)
			}

			return s.updateStatus(func(currStatus *radixv1.RadixDNSAliasStatus) {
				*currStatus = status
			})
		}
	}

	return nil
}
