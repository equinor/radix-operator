package applicationconfig

import (
	"context"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncBuildSecrets(ctx context.Context) error {
	appNamespace := utils.GetAppNamespace(app.config.Name)
	isSecretExist := app.kubeutil.SecretExists(ctx, appNamespace, defaults.BuildSecretsName)

	if app.config.Spec.Build == nil {
		if isSecretExist {
			// Delete build secret
			err := app.kubeutil.DeleteSecret(ctx, appNamespace, defaults.BuildSecretsName)
			if err != nil {
				return err
			}
		}
		err := garbageCollectAccessToBuildSecrets(ctx, app)
		if err != nil {
			return err
		}
	} else {
		buildSecrets := app.config.Spec.Build.Secrets
		if !isSecretExist {
			// Create build secret
			err := app.initializeBuildSecret(ctx, appNamespace, defaults.BuildSecretsName, buildSecrets)
			if err != nil {
				return err
			}
		} else {
			// Update build secret if there is any change
			err := app.updateBuildSecret(ctx, appNamespace, defaults.BuildSecretsName, buildSecrets)
			if err != nil {
				return err
			}
		}

		// Grant access to build secret (RBAC)
		err := app.grantAccessToBuildSecrets(ctx, appNamespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *ApplicationConfig) initializeBuildSecret(ctx context.Context, namespace, name string, buildSecrets []string) error {
	data := make(map[string][]byte)
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	for _, buildSecret := range buildSecrets {
		data[buildSecret] = defaultValue
	}

	secret := getBuildSecretForData(app.config.Name, namespace, name, data)
	_, err := app.kubeutil.ApplySecret(ctx, namespace, secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}
	return nil
}

func (app *ApplicationConfig) updateBuildSecret(ctx context.Context, namespace, name string, buildSecrets []string) error {
	secret, err := app.kubeutil.GetSecret(ctx, namespace, name)
	if err != nil {
		return err
	}

	orphanRemoved := removeOrphanedSecrets(secret, buildSecrets)
	secretAppended := appendSecrets(secret, buildSecrets)
	if !orphanRemoved && !secretAppended {
		// Secret definition may have changed, but not data
		secret = getBuildSecretForData(app.config.Name, namespace, name, secret.Data)
	}

	_, err = app.kubeutil.ApplySecret(ctx, namespace, secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
	}

	return nil
}

func removeOrphanedSecrets(buildSecrets *corev1.Secret, secrets []string) bool {
	orphanRemoved := false
	for secretName := range buildSecrets.Data {
		if !commonUtils.ContainsString(secrets, secretName) {
			delete(buildSecrets.Data, secretName)
			orphanRemoved = true
		}
	}

	return orphanRemoved
}

func appendSecrets(buildSecrets *corev1.Secret, secrets []string) bool {
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	if buildSecrets.Data == nil {
		data := make(map[string][]byte)
		buildSecrets.Data = data
	}

	secretAppended := false
	for _, secretName := range secrets {
		if _, ok := buildSecrets.Data[secretName]; !ok {
			buildSecrets.Data[secretName] = defaultValue
			secretAppended = true
		}
	}

	return secretAppended
}

func getBuildSecretForData(appName, namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		Data: data,
	}
}
