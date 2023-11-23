package applicationconfig

import (
	"fmt"
	"slices"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrSyncSubPipelineServiceAccount    = fmt.Errorf("failed to sync serviceaccount")
	ErrCleanupSubPipelineServiceAccount = fmt.Errorf("failed to cleanup serviceaccount")
)

// syncSubPipelineServiceAccounts creates, updates and cleans up service accounts used by sub-pipelines / tekton
func (app *ApplicationConfig) syncSubPipelineServiceAccounts() error {

	if err := app.applySubPipelineServiceAccounts(); err != nil {
		return err
	}

	if err := app.gcSubPipelineServiceAccounts(); err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) applySubPipelineServiceAccounts() error {
	appNs := utils.GetAppNamespace(app.registration.Name)

	for _, env := range app.config.Spec.Environments {
		saName := utils.GetSubPipelineServiceAccountName(env.Name)

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: appNs,

				Labels: map[string]string{
					kube.IsServiceAccountForSubPipelineLabel: "true",
					kube.RadixEnvLabel:                       env.Name,
				},
			},
		}

		_, err := app.kubeutil.ApplyServiceAccount(sa)

		if err != nil {
			return fmt.Errorf("%w: %s/%s: %w", ErrSyncSubPipelineServiceAccount, appNs, saName, err)
		}
	}

	return nil
}

func (app *ApplicationConfig) gcSubPipelineServiceAccounts() error {
	appNs := utils.GetAppNamespace(app.registration.Name)
	accounts, err := app.kubeutil.ListServiceAccountsWithSelector(appNs, fmt.Sprintf("%s=%s", kube.IsServiceAccountForSubPipelineLabel, true))

	if err != nil {
		return fmt.Errorf("failed to list: %w: %w", ErrCleanupSubPipelineServiceAccount, err)
	}

	for _, sa := range accounts {
		targetEnv := sa.Labels[kube.RadixEnvLabel]

		exists := slices.ContainsFunc(app.config.Spec.Environments, func(e v1.Environment) bool {
			return e.Name == targetEnv
		})

		if !exists {
			err = app.kubeutil.DeleteServiceAccount(appNs, sa.Name)

			if err != nil {
				return fmt.Errorf("%w: %s/%s: %w", ErrCleanupSubPipelineServiceAccount, appNs, sa.Name, err)
			}
		}
	}

	return nil
}
