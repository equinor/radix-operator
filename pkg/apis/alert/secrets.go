package alert

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (syncer *alertSyncer) createOrUpdateSecret(ctx context.Context) error {
	secretName, ns := GetAlertSecretName(syncer.radixAlert.Name), syncer.radixAlert.Namespace

	secret, err := syncer.kubeUtil.GetSecret(ctx, ns, secretName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if secret == nil {
		secret = &v1.Secret{
			Type: v1.SecretType("Opaque"),
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
		}
	} else {
		syncer.removedOrphanedSecretKeys(secret)
	}

	syncer.setSecretCommonProps(secret)

	_, err = syncer.kubeUtil.ApplySecret(ctx, ns, secret)
	return err
}

func (syncer *alertSyncer) setSecretCommonProps(secret *v1.Secret) {
	secret.OwnerReferences = syncer.getOwnerReference()

	labels := map[string]string{}
	if appName, found := syncer.radixAlert.Labels[kube.RadixAppLabel]; found {
		labels[kube.RadixAppLabel] = appName
	}
	secret.Labels = labels
}

func (syncer *alertSyncer) removedOrphanedSecretKeys(secret *v1.Secret) {
	expectedKeys := map[string]interface{}{}

	// Secret keys related to receiver configuration
	for receiverName := range syncer.radixAlert.Spec.Receivers {
		expectedKeys[GetSlackConfigSecretKeyName(receiverName)] = nil
	}

	for key := range secret.Data {
		if _, found := expectedKeys[key]; !found {
			delete(secret.Data, key)
		}
	}

}

// GetAlertSecretName returns name of secret used to store configuration for the RadixAlert
func GetAlertSecretName(alertName string) string {
	return fmt.Sprintf("%s-alert-config", alertName)
}

// GetSlackConfigSecretKeyName returns the secret key name to store Slack webhook URL for a given receiver
func GetSlackConfigSecretKeyName(receiverName string) string {
	return fmt.Sprintf("receiver-%s-slackurl", receiverName)
}
