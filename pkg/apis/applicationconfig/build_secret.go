package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (app *ApplicationConfig) syncBuildSecrets() error {
	if app.config.Spec.Build == nil || len(app.config.Spec.Build.Secrets) == 0 {
		return nil
	}

	appNamespace := utils.GetAppNamespace(app.config.Name)
	if len(app.config.Spec.Build.Secrets) > 0 {
		if !app.kubeutil.SecretExists(appNamespace, defaults.BuildSecretsName) {
			err := app.applyEmptyBuildSecret(appNamespace, defaults.BuildSecretsName, app.config.Spec.Build.Secrets)
			if err != nil {
				return err
			}

			err = app.grantPipelineAccessToBuildSecrets(appNamespace)
			if err != nil {
				return err
			}

		} else {
			err := removeOrphanedSecrets(app.kubeclient, appNamespace, defaults.BuildSecretsName, app.config.Spec.Build.Secrets)
			if err != nil {
				return err
			}
		}

	} else {
		err := garbageCollectBuildSecretsNoLongerInSpec(app.kubeclient, app.kubeutil, appNamespace, defaults.BuildSecretsName)
		if err != nil {
			return err
		}
	}

	return nil
}

func garbageCollectBuildSecretsNoLongerInSpec(kubeclient kubernetes.Interface, kubeutil *kube.Kube, namespace, name string) error {
	if kubeutil.SecretExists(namespace, name) {
		err := kubeclient.CoreV1().Secrets(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		garbageCollectAccessToBuildSecrets(kubeclient, namespace, name)
	}

	return nil
}

func garbageCollectAccessToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	err := garbageCollectAppAdminRoleBindingToBuildSecrets(kubeclient, namespace, name)
	if err != nil {
		return err
	}

	err = garbageCollectAppAdminRoleToBuildSecrets(kubeclient, namespace, name)
	if err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) grantPipelineAccessToBuildSecrets(namespace string) error {
	role := roleAppAdminBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingPipelineToBuildSecrets(app.GetRadixRegistration(), role)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (app *ApplicationConfig) applyEmptyBuildSecret(namespace, name string, buildSecrets []string) error {
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

func removeOrphanedSecrets(kubeclient kubernetes.Interface, ns, secretName string, secrets []string) error {
	secret, err := kubeclient.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	orphanRemoved := false
	for secretName := range secret.Data {
		if !slice.ContainsString(secrets, secretName) {
			delete(secret.Data, secretName)
			orphanRemoved = true
		}
	}

	if orphanRemoved {
		_, err = kubeclient.CoreV1().Secrets(ns).Update(secret)
		if err != nil {
			return err
		}
	}

	return nil
}
