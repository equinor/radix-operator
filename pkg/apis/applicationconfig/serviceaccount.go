package applicationconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errSyncSubPipelineServiceAccount    = fmt.Errorf("failed to sync sub-pipeline service accounts")
	errCleanupSubPipelineServiceAccount = fmt.Errorf("failed to garbage collect sub-pipeline service accounts")
)

// syncSubPipelineServiceAccounts creates, updates and cleans up service accounts used by sub-pipelines / tekton
func (app *ApplicationConfig) syncSubPipelineServiceAccounts(ctx context.Context) error {

	if err := app.applySubPipelineServiceAccounts(ctx); err != nil {
		return err
	}

	if err := app.gcSubPipelineServiceAccounts(ctx); err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) applySubPipelineServiceAccounts(ctx context.Context) error {
	appNs := utils.GetAppNamespace(app.registration.Name)

	for _, env := range app.config.Spec.Environments {
		saName := utils.GetSubPipelineServiceAccountName(env.Name)

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: appNs,
				Labels: radixlabels.Merge(
					radixlabels.ForServiceAccountIsForSubPipeline(),
					radixlabels.ForEnvironmentName(env.Name),
				),
			},
		}

		_, err := app.kubeutil.ApplyServiceAccount(ctx, sa)
		if err != nil {
			return fmt.Errorf("%w: service account %s/%s: %w", errSyncSubPipelineServiceAccount, appNs, saName, err)
		}
	}

	return nil
}

func (app *ApplicationConfig) gcSubPipelineServiceAccounts(ctx context.Context) error {
	appNs := utils.GetAppNamespace(app.registration.Name)
	accounts, err := app.kubeutil.ListServiceAccountsWithSelector(ctx, appNs, radixlabels.ForServiceAccountIsForSubPipeline().AsSelector().String())
	if err != nil {
		return fmt.Errorf("%w: failed to list: %w", errCleanupSubPipelineServiceAccount, err)
	}

	for _, sa := range accounts {
		targetEnv := sa.Labels[kube.RadixEnvLabel]

		envExists := slices.ContainsFunc(app.config.Spec.Environments, func(e radixv1.Environment) bool {
			return e.Name == targetEnv
		})
		if envExists {
			continue
		}

		// Delete service-accounts that don't have a matching environment
		err = app.kubeutil.DeleteServiceAccount(ctx, appNs, sa.Name)
		if err != nil {
			return fmt.Errorf("%w: service account %s/%s: %w", errCleanupSubPipelineServiceAccount, appNs, sa.Name, err)
		}
	}

	return nil
}
