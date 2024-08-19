package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	originalSecret, desiredSecret, derivedPublicKey, err := app.getCurrentAndDesiredGitPrivateDeployKeySecret(ctx)
	if err != nil {
		return err
	}

	originalCm, desiredCm, err := app.getCurrentAndDesiredGitPublicDeployKeyConfigMap(ctx, derivedPublicKey)
	if err != nil {
		return err
	}

	if originalSecret != nil {
		if _, err := app.kubeutil.UpdateSecret(ctx, originalSecret, desiredSecret); err != nil {
			return err
		}
	} else {
		if _, err := app.kubeutil.CreateSecret(ctx, desiredSecret.Namespace, desiredSecret); err != nil {
			return err
		}
	}

	if originalCm != nil {
		if _, err = app.kubeutil.UpdateConfigMap(ctx, originalCm, desiredCm); err != nil {
			return err
		}
	} else {
		if _, err := app.kubeutil.CreateConfigMap(ctx, desiredCm.Namespace, desiredCm); err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) getCurrentAndDesiredGitPrivateDeployKeySecret(ctx context.Context) (original, desired *corev1.Secret, derivedPublicKey string, err error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	originalInternal, err := app.kubeutil.GetSecret(ctx, namespace, defaults.GitPrivateKeySecretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, "", err
		}
		original = nil
		desired = &corev1.Secret{
			Type: "Opaque",
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaults.GitPrivateKeySecretName,
				Namespace: namespace,
			},
		}
	} else {
		desired = originalInternal.DeepCopy()
		original = originalInternal
	}

	knownHostsSecret, err := app.kubeutil.GetSecret(ctx, corev1.NamespaceDefault, "radix-known-hosts-git")
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get known hosts secret: %w", err)
	}

	var (
		privateKey []byte
	)
	switch {
	case original != nil && secretHasGitPrivateDeployKey(original):
		privateKey = original.Data[defaults.GitPrivateKeySecretKey]
		keypair, err := utils.DeriveDeployKeyFromPrivateKey(string(privateKey))
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to parse deploy key from existing secret: %w", err)
		}
		derivedPublicKey = keypair.PublicKey
	case len(app.registration.Spec.DeployKey) > 0:
		privateKey = []byte(app.registration.Spec.DeployKey)
		derivedPublicKey = app.registration.Spec.DeployKeyPublic
	default:
		keypair, err := utils.GenerateDeployKey()
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to generate new git deploy key: %w", err)
		}
		privateKey = []byte(keypair.PrivateKey)
		derivedPublicKey = keypair.PublicKey
	}

	desired.ObjectMeta.Labels = labels.ForApplicationName(app.registration.Name) // Required when restoring with Velero. We only restore secrets with the "radix-app" label
	desired.Data = map[string][]byte{
		defaults.GitPrivateKeySecretKey: privateKey,
		"known_hosts":                   knownHostsSecret.Data["known_hosts"],
	}

	return original, desired, derivedPublicKey, nil
}

func (app *Application) getCurrentAndDesiredGitPublicDeployKeyConfigMap(ctx context.Context, publicKey string) (original, desired *corev1.ConfigMap, err error) {
	namespace := utils.GetAppNamespace(app.registration.Name)
	originalInternal, err := app.kubeutil.GetConfigMap(ctx, namespace, defaults.GitPublicKeyConfigMapName)
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
		desired = originalInternal.DeepCopy()
		original = originalInternal
	}

	desired.Data = map[string]string{
		defaults.GitPublicKeyConfigMapKey: publicKey,
	}

	return original, desired, nil
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
		_, err = app.kubeutil.ApplySecret(ctx, buildNamespace, secret)
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
