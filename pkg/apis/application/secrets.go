package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	knownHostsSecretKey = "known_hosts"
)

// ApplySecretsForPipelines creates secrets needed by pipeline to run
func (app *Application) applySecretsForPipelines(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("Apply secrets for pipelines")

	if err := app.applyGitDeployKeyToBuildNamespace(ctx); err != nil {
		return fmt.Errorf("failed to apply pipeline git deploy keys: %w", err)
	}

	if err := app.applyContainerRegistryCredentialSecretsToAppNamespace(ctx); err != nil {
		return fmt.Errorf("failed to apply pipeline service principal ACR secrets: %w", err)
	}

	return nil
}

func (app *Application) applyGitDeployKeyToBuildNamespace(ctx context.Context) error {
	currentSecret, desiredSecret, derivedPublicKey, err := app.getCurrentAndDesiredGitPrivateDeployKeySecret(ctx)
	if err != nil {
		return err
	}

	currentCm, desiredCm, err := app.getCurrentAndDesiredGitPublicDeployKeyConfigMap(ctx, derivedPublicKey)
	if err != nil {
		return err
	}

	if currentSecret != nil {
		if _, err := app.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
			return err
		}
	} else {
		if _, err := app.kubeutil.CreateSecret(ctx, desiredSecret.Namespace, desiredSecret); err != nil {
			return err
		}
	}

	if currentCm != nil {
		if _, err = app.kubeutil.UpdateConfigMap(ctx, currentCm, desiredCm); err != nil {
			return err
		}
	} else {
		if _, err := app.kubeutil.CreateConfigMap(ctx, desiredCm.Namespace, desiredCm); err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) getCurrentAndDesiredGitPrivateDeployKeySecret(ctx context.Context) (current, desired *corev1.Secret, derivedPublicKey string, err error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	// Cannot assign `current` directly from GetSecret since kube client returns a non-nil value even when an error is returned
	currentInternal, err := app.kubeutil.GetSecret(ctx, namespace, defaults.GitPrivateKeySecretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, "", err
		}
		desired = &corev1.Secret{
			Type: "Opaque",
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.GitPrivateKeySecretName,
				Namespace: namespace,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	knownHostsSecret, err := app.kubeutil.GetSecret(ctx, corev1.NamespaceDefault, "radix-known-hosts-git")
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get known hosts secret: %w", err)
	}

	deployKey, err := getExistingOrGenerateNewDeployKey(current, app.registration)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get deploy key: %w", err)
	}

	desired.ObjectMeta.Labels = labels.ForApplicationName(app.registration.Name) // Required when restoring with Velero. We only restore secrets with the "radix-app" label
	desired.Data = map[string][]byte{
		defaults.GitPrivateKeySecretKey: []byte(deployKey.PrivateKey),
		knownHostsSecretKey:             knownHostsSecret.Data[knownHostsSecretKey],
	}

	return current, desired, deployKey.PublicKey, nil
}

func getExistingOrGenerateNewDeployKey(fromSecret *corev1.Secret, fromRadixRegistration *v1.RadixRegistration) (*utils.DeployKey, error) {
	switch {
	case fromSecret != nil && secretHasGitPrivateDeployKey(fromSecret):
		privateKey := fromSecret.Data[defaults.GitPrivateKeySecretKey]
		keypair, err := utils.DeriveDeployKeyFromPrivateKey(string(privateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse deploy key from existing secret: %w", err)
		}
		return keypair, nil
	default:
		keypair, err := utils.GenerateDeployKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate new git deploy key: %w", err)
		}
		return keypair, nil
	}
}

func (app *Application) getCurrentAndDesiredGitPublicDeployKeyConfigMap(ctx context.Context, publicKey string) (current, desired *corev1.ConfigMap, err error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	// Cannot assign `current` directly from GetConfigMap since kube client returns a non-nil value even when an error is returned
	currentInternal, err := app.kubeutil.GetConfigMap(ctx, namespace, defaults.GitPublicKeyConfigMapName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, err
		}
		desired = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.GitPublicKeyConfigMapName,
				Namespace: namespace,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	desired.Data = map[string]string{
		defaults.GitPublicKeyConfigMapKey: publicKey,
	}

	return current, desired, nil
}

// applyWebhookSharedSecret ensures a Kubernetes secret holding the GitHub webhook shared secret exists in the app namespace.
// The secret is the source of truth for the shared secret. It is only seeded once (migrating the value from the deprecated
// RadixRegistrationSpec.SharedSecret field, or generating a new random value) and is not overwritten by the operator afterwards
// as long as it holds valid data, so that regenerations performed through the api-server are preserved. If the secret is missing
// or does not contain valid data, it is (re)seeded.
func (app *Application) applyWebhookSharedSecret(ctx context.Context) error {
	namespace := utils.GetAppNamespace(app.registration.Name)

	// Cannot assign `current` directly from GetSecret since kube client returns a non-nil value even when an error is returned
	current, err := app.kubeutil.GetSecret(ctx, namespace, defaults.WebhookSharedSecretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		current = nil
	}

	if current != nil && webhookSharedSecretHasValidData(current) {
		// Secret already exists with valid data and is the source of truth - do not overwrite it.
		return nil
	}

	// TODO: When all Secrets have been created and seeded, remove the deprecated Spec.SharedSecret field from RadixRegistration and stop seeding from it.
	sharedSecret := strings.TrimSpace(app.registration.Spec.SharedSecret)
	if sharedSecret == "" {
		sharedSecret = utils.RandString(20)
	}
	encodedSharedSecret := base64.StdEncoding.EncodeToString([]byte(sharedSecret))

	if current != nil {
		desired := current.DeepCopy()
		desired.ObjectMeta.Labels = labels.ForApplicationName(app.registration.Name)
		if desired.Data == nil {
			desired.Data = map[string][]byte{}
		}
		desired.Data[defaults.WebhookSharedSecretKey] = []byte(encodedSharedSecret)
		_, err := app.kubeutil.UpdateSecret(ctx, current, desired)
		return err
	}

	desired := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.WebhookSharedSecretName,
			Namespace: namespace,
			Labels:    labels.ForApplicationName(app.registration.Name),
		},
		Data: map[string][]byte{
			defaults.WebhookSharedSecretKey: []byte(encodedSharedSecret),
		},
	}

	_, err = app.kubeutil.CreateSecret(ctx, namespace, desired)
	return err
}

// webhookSharedSecretHasValidData returns true if the secret contains a non-empty, base64-encoded shared secret value.
func webhookSharedSecretHasValidData(secret *corev1.Secret) bool {
	encodedSharedSecret, ok := secret.Data[defaults.WebhookSharedSecretKey]
	if !ok || len(encodedSharedSecret) == 0 {
		return false
	}
	decodedSharedSecret, err := base64.StdEncoding.DecodeString(string(encodedSharedSecret))
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(decodedSharedSecret))) > 0
}

func (app *Application) applyContainerRegistryCredentialSecretsToAppNamespace(ctx context.Context) error {
	appNamespace := utils.GetAppNamespace(app.registration.Name)

	for _, secretName := range []string{defaults.AzureACRServicePrincipleBuildahSecretName, defaults.AzureACRTokenPasswordAppRegistrySecretName} {
		err := app.copySecretData(ctx, corev1.NamespaceDefault, appNamespace, secretName)
		if err != nil {
			return fmt.Errorf("failed to sync secret %s: %w", secretName, err)
		}
	}

	return nil
}

func (app *Application) copySecretData(ctx context.Context, sourceNamespace, targetNamespace, secretName string) error {
	sourceSecret, err := app.kubeutil.GetSecret(ctx, sourceNamespace, secretName)
	if err != nil {
		return fmt.Errorf("failed to get source secret: %w", err)
	}

	var currentSecret, desiredSecret *corev1.Secret
	currentSecret, err = app.kubeutil.GetSecret(ctx, targetNamespace, secretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get current target secret: %w", err)
		}
		desiredSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: targetNamespace,
			},
			Type: sourceSecret.Type,
		}
		currentSecret = nil
	} else {
		desiredSecret = currentSecret.DeepCopy()
	}

	desiredSecret.Data = sourceSecret.Data

	if currentSecret == nil {
		if _, err := app.kubeutil.CreateSecret(ctx, targetNamespace, desiredSecret); err != nil {
			return fmt.Errorf("failed to create target secret: %w", err)
		}
		return nil
	}

	if _, err := app.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
		return fmt.Errorf("failed to update target secret: %w", err)
	}

	return nil
}

func secretHasGitPrivateDeployKey(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}
	return len(strings.TrimSpace(string(secret.Data[defaults.GitPrivateKeySecretKey]))) > 0
}
