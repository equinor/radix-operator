package utils

import (
	"context"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func UpdateRadixEnvironmentStatus(ctx context.Context, radixClient versioned.Interface, radixApplication *v1.RadixApplication, radixEnvironment *v1.RadixEnvironment, time metav1.Time) error {
	radixEnvironmentInterface := radixClient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		name := radixEnvironment.GetName()
		currentEnv, err := radixEnvironmentInterface.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatus(radixApplication, radixEnvironment.Spec.EnvName, &currentEnv.Status, time)
		_, err = radixEnvironmentInterface.UpdateStatus(ctx, currentEnv, metav1.UpdateOptions{})
		return err
	})
}

func changeStatus(radixApplication *v1.RadixApplication, envName string, currStatus *v1.RadixEnvironmentStatus, syncTime metav1.Time) {
	isOrphaned := !existsInAppConfig(radixApplication, envName)
	if isOrphaned && currStatus.OrphanedTimestamp == nil {
		currStatus.OrphanedTimestamp = pointers.Ptr(syncTime)
	} else if !isOrphaned && currStatus.OrphanedTimestamp != nil {
		currStatus.OrphanedTimestamp = nil
	}
	currStatus.Orphaned = isOrphaned
	// time is parameterized for testability
	currStatus.Reconciled = syncTime
}

func existsInAppConfig(app *v1.RadixApplication, envName string) bool {
	if app == nil {
		return false
	}
	for _, appEnv := range app.Spec.Environments {
		if appEnv.Name == envName {
			return true
		}
	}
	return false
}
