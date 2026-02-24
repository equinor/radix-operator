package alert

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (syncer *alertSyncer) reconcileSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetAlertSecretName(syncer.radixAlert.Name),
			Namespace: syncer.radixAlert.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, secret, func() error {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Type = corev1.SecretTypeOpaque
		if appName, exist := syncer.radixAlert.Labels[kube.RadixAppLabel]; exist {
			secret.Labels[kube.RadixAppLabel] = appName
		} else {
			delete(secret.Labels, kube.RadixAppLabel)
		}

		syncer.removedOrphanedSecretKeys(secret)

		return controllerutil.SetControllerReference(syncer.radixAlert, secret, syncer.dynamicClient.Scheme())
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile secret: %w", err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("op", string(op)).Msg("reconcile secret for alert configuration")
	}

	return nil
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
