package application

import (
	"context"
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

	if err := app.applyServicePrincipalACRSecretToBuildNamespace(ctx); err != nil {
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
	case len(fromRadixRegistration.Spec.DeployKey) > 0:
		return &utils.DeployKey{
			PrivateKey: fromRadixRegistration.Spec.DeployKey,
			PublicKey:  fromRadixRegistration.Spec.DeployKeyPublic,
		}, nil
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

func (app *Application) applyServicePrincipalACRSecretToBuildNamespace(ctx context.Context) error {
	buildNamespace := utils.GetAppNamespace(app.registration.Name)
	servicePrincipalSecretForBuild, err := app.createNewServicePrincipalACRSecret(ctx, buildNamespace, defaults.AzureACRServicePrincipleSecretName)
	if err != nil {
		return err
	}

	servicePrincipalSecretForBuildahBuild, err := app.createNewServicePrincipalACRSecret(ctx, buildNamespace, defaults.AzureACRServicePrincipleBuildahSecretName)
	if err != nil {
		return err
	}

	tokenSecretForAppRegistry, err := app.createNewServicePrincipalACRSecret(ctx, buildNamespace, defaults.AzureACRTokenPasswordAppRegistrySecretName)
	if err != nil {
		return err
	}

	for _, secret := range []*corev1.Secret{servicePrincipalSecretForBuild, servicePrincipalSecretForBuildahBuild, tokenSecretForAppRegistry} {
		_, err = app.kubeutil.ApplySecret(ctx, buildNamespace, secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) createNewServicePrincipalACRSecret(ctx context.Context, namespace, secretName string) (*corev1.Secret, error) {
	servicePrincipalSecret, err := app.kubeutil.GetSecret(ctx, corev1.NamespaceDefault, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s from default namespace: %w", secretName, err)
	}

	data := map[string][]byte{}
	for key := range servicePrincipalSecret.Data {
		data[key] = servicePrincipalSecret.Data[key]
	}

	secret := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: data,
	}
	return &secret, nil
}

func secretHasGitPrivateDeployKey(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}
	return len(strings.TrimSpace(string(secret.Data[defaults.GitPrivateKeySecretKey]))) > 0
}
