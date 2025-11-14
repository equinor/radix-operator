package applicationconfig

import (
	"context"
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (app *ApplicationConfig) syncStatus(ctx context.Context, reconcileErr error) error {
	err := app.updateStatus(ctx, func(currStatus *radixv1.RadixApplicationStatus) {
		currStatus.Reconciled = metav1.Now()
		currStatus.ObservedGeneration = app.config.Generation

		if reconcileErr != nil {
			currStatus.ReconcileStatus = radixv1.RadixApplicationReconcileFailed
			currStatus.Message = reconcileErr.Error()
		} else {
			currStatus.ReconcileStatus = radixv1.RadixApplicationReconcileSucceeded
			currStatus.Message = ""
		}
	})
	if err != nil {
		return fmt.Errorf("failed to sync status: %w", err)
	}

	return reconcileErr
}

func (app *ApplicationConfig) updateStatus(ctx context.Context, changeStatusFunc func(currStatus *radixv1.RadixApplicationStatus)) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateObj := app.config.DeepCopy()
		changeStatusFunc(&updateObj.Status)
		updateObj, err := app.radixclient.RadixV1().RadixApplications(app.config.GetNamespace()).UpdateStatus(ctx, updateObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		app.config = updateObj
		return nil
	})
	return err
}
