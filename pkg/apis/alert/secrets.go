package alert

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (syncer *alertSyncer) createOrUpdateSecret(ctx context.Context) error {
	current, desired, err := syncer.getCurrentAndDesiredAlertSecret(ctx)
	if err != nil {
		return err
	}

	if current != nil {
		_, err = syncer.kubeUtil.UpdateSecret(ctx, current, desired)
		return err
	}

	_, err = syncer.kubeUtil.CreateSecret(ctx, desired.Namespace, desired)
	return err
}

func (syncer *alertSyncer) getCurrentAndDesiredAlertSecret(ctx context.Context) (current, desired *corev1.Secret, err error) {
	secretName, ns := GetAlertSecretName(syncer.radixAlert.Name), syncer.radixAlert.Namespace
	currentInternal, err := syncer.kubeUtil.GetSecret(ctx, ns, secretName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, err
		}
		desired = &corev1.Secret{
			Type: corev1.SecretType("Opaque"),
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	syncer.removedOrphanedSecretKeys(desired)
	syncer.setSecretCommonProps(desired)
	return current, desired, nil
}

func (syncer *alertSyncer) setSecretCommonProps(secret *corev1.Secret) {
	secret.OwnerReferences = syncer.getOwnerReference()

	labels := map[string]string{}
	if appName, found := syncer.radixAlert.Labels[kube.RadixAppLabel]; found {
		labels[kube.RadixAppLabel] = appName
	}
	secret.Labels = labels
}

func (syncer *alertSyncer) removedOrphanedSecretKeys(secret *corev1.Secret) {
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
