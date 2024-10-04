package utils

import (
	"context"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func UpdateRadixEnvironmentStatus(ctx context.Context, radixClient versioned.Interface, radixApplication *v1.RadixApplication, radixEnvironment *v1.RadixEnvironment, time v2.Time) error {
	radixEnvironmentInterface := radixClient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		name := radixEnvironment.GetName()
		currentEnv, err := radixEnvironmentInterface.Get(ctx, name, v2.GetOptions{})
		if err != nil {
			return err
		}
		changeStatus(radixApplication, radixEnvironment.Spec.EnvName, &currentEnv.Status, time)
		_, err = radixEnvironmentInterface.UpdateStatus(ctx, currentEnv, v2.UpdateOptions{})
		return err
	})
}

func changeStatus(radixApplication *v1.RadixApplication, envName string, currStatus *v1.RadixEnvironmentStatus, time v2.Time) {
	isOrphaned := !existsInAppConfig(radixApplication, envName)
	if isOrphaned && len(currStatus.OrphanedTimestamp) == 0 {
		currStatus.OrphanedTimestamp = utils.FormatTimestamp(time.Time)
	} else if !isOrphaned && len(currStatus.OrphanedTimestamp) > 0 {
		currStatus.OrphanedTimestamp = ""
	}
	currStatus.Orphaned = isOrphaned
	// time is parameterized for testability
	currStatus.Reconciled = time
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
