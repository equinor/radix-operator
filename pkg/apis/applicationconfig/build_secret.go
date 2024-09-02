package applicationconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncBuildSecrets(ctx context.Context) error {
	if app.config.Spec.Build == nil || len(app.config.Spec.Build.Secrets) == 0 {
		if err := app.garbageCollectBuildSecrets(ctx); err != nil {
			return fmt.Errorf("failed to garbage collect build secret: %w", err)
		}

		if err := app.garbageCollectAccessToBuildSecrets(ctx); err != nil {
			return fmt.Errorf("failed to garbage collect access to build secret: %w", err)
		}
	} else {
		currentSecret, desiredSecret, err := app.getCurrentAndDesiredBuildSecret(ctx)
		if err != nil {
			return fmt.Errorf("failed get current and desired build secret: %w", err)
		}

		if currentSecret != nil {
			if _, err := app.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
				return fmt.Errorf("failed to update build secret: %w", err)
			}
		} else {
			if _, err := app.kubeutil.CreateSecret(ctx, desiredSecret.Namespace, desiredSecret); err != nil {
				return fmt.Errorf("failed to create build secret: %w", err)
			}
		}

		if err := app.grantAccessToBuildSecrets(ctx); err != nil {
			return fmt.Errorf("failed to grant access to build secret: %w", err)
		}
	}

	return nil
}

func (app *ApplicationConfig) getCurrentAndDesiredBuildSecret(ctx context.Context) (current, desired *corev1.Secret, err error) {
	ns := utils.GetAppNamespace(app.config.Name)
	currentInternal, err := app.kubeutil.GetSecret(ctx, ns, defaults.BuildSecretsName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, err
		}
		desired = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.BuildSecretsName,
				Namespace: ns,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	desired.Labels = map[string]string{
		kube.RadixAppLabel: app.config.Name,
	}

	setBuildSecretData(desired, app.config.Spec.Build.Secrets)

	return current, desired, nil
}

func (app *ApplicationConfig) garbageCollectBuildSecrets(ctx context.Context) error {
	secret, err := app.kubeutil.GetSecret(ctx, utils.GetAppNamespace(app.config.Name), defaults.BuildSecretsName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return app.kubeutil.DeleteSecret(ctx, secret.Namespace, secret.Name)
}

func setBuildSecretData(buildSecret *corev1.Secret, buildSecretKeys []string) {
	removeOrphanedBuildSecretKeys(buildSecret, buildSecretKeys)
	appendMissingBuildSecretKeys(buildSecret, buildSecretKeys)
}

func removeOrphanedBuildSecretKeys(buildSecret *corev1.Secret, buildSecretKeys []string) bool {
	orphanRemoved := false
	for secretName := range buildSecret.Data {
		if !slices.Contains(buildSecretKeys, secretName) {
			delete(buildSecret.Data, secretName)
			orphanRemoved = true
		}
	}

	return orphanRemoved
}

func appendMissingBuildSecretKeys(buildSecrets *corev1.Secret, buildSecretKeys []string) bool {
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	if buildSecrets.Data == nil {
		data := make(map[string][]byte)
		buildSecrets.Data = data
	}

	secretAppended := false
	for _, secretName := range buildSecretKeys {
		if _, ok := buildSecrets.Data[secretName]; !ok {
			buildSecrets.Data[secretName] = defaultValue
			secretAppended = true
		}
	}

	return secretAppended
}
