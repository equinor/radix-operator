package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncBuildSecrets() error {
	appNamespace := utils.GetAppNamespace(app.config.Name)

	var buildSecrets []string
	if app.config.Spec.Build != nil {
		buildSecrets = app.config.Spec.Build.Secrets
	}

	if !app.kubeutil.SecretExists(appNamespace, defaults.BuildSecretsName) {
		err := app.initializeBuildSecret(appNamespace, defaults.BuildSecretsName, buildSecrets)
		if err != nil {
			return err
		}

		err = app.grantAccessToBuildSecrets(appNamespace)
		if err != nil {
			return err
		}

	} else {
		err := app.updateBuildSecret(appNamespace, defaults.BuildSecretsName, buildSecrets)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *ApplicationConfig) initializeBuildSecret(namespace, name string, buildSecrets []string) error {
	data := make(map[string][]byte)
	defaultValue := []byte(defaults.BuildSecretDefaultData)

	for _, buildSecret := range buildSecrets {
		data[buildSecret] = defaultValue
	}

	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	_, err := app.kubeutil.ApplySecret(namespace, &secret)
	if err != nil {
		log.Warnf("Failed to create build secret %s in %s", name, namespace)
		return err
	}
	return nil
}

func (app *ApplicationConfig) updateBuildSecret(namespace, name string, buildSecrets []string) error {
	secret, err := app.kubeutil.GetSecret(namespace, name)
	if err != nil {
		return err
	}

	orphanRemoved := removeOrphanedSecrets(secret, buildSecrets)
	secretAppended := appendSecrets(secret, buildSecrets)
	if orphanRemoved || secretAppended {
		_, err = app.kubeutil.ApplySecret(namespace, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeOrphanedSecrets(buildSecrets *corev1.Secret, secrets []string) bool {
	orphanRemoved := false
	for secretName := range buildSecrets.Data {
		if !slice.ContainsString(secrets, secretName) {
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
