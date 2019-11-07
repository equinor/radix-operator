package applicationconfig

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) syncBuildSecrets() error {
	if app.config.Spec.Build == nil || len(app.config.Spec.Build.Secrets) == 0 {
		return nil
	}

	err := app.garbageCollectBuildSecretsNoLongerInSpec()
	if err != nil {
		return err
	}

	appNamespace := utils.GetAppNamespace(app.config.Name)
	for _, secretName := range app.config.Spec.Build.Secrets {
		secret, err := app.kubeclient.CoreV1().Secrets(appNamespace).Get(secretName, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		// Only create secret, if not present. Else, do nothing
		if errors.IsNotFound(err) || secret == nil {
			err = applyEmptyBuildSecret(app.kubeutil, appNamespace, secretName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (app *ApplicationConfig) garbageCollectBuildSecretsNoLongerInSpec() error {
	appNamespace := utils.GetAppNamespace(app.config.Name)
	secrets, err := app.kubeclient.CoreV1().Secrets(appNamespace).List(metav1.ListOptions{
		LabelSelector: kube.RadixBuildSecretLabel,
	})
	if err != nil {
		return err
	}

	for _, exisitingSecret := range secrets.Items {
		garbageCollect := true

		for _, buildSecret := range app.config.Spec.Build.Secrets {
			if strings.EqualFold(buildSecret, exisitingSecret.Name) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = app.kubeclient.CoreV1().Secrets(appNamespace).Delete(exisitingSecret.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func applyEmptyBuildSecret(kubeutil *kube.Kube, namespace, name string) error {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				kube.RadixBuildSecretLabel: name,
			},
		},
		Data: map[string][]byte{},
	}

	_, err := kubeutil.ApplySecret(namespace, &secret)
	if err != nil {
		log.Warnf("Failed to create build secret %s in %s", name, namespace)
		return err
	}
	return nil
}
