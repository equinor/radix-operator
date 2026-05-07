package kubequery

import (
	"context"
	"encoding/json"
	"time"

	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const RadixMetadataAnnotation = "radix.equinor.com/secret-metadata"

type SecretMetadata map[string]SecretMetadataItem
type SecretMetadataItem struct {
	Updated *time.Time `json:"updated,omitempty"`
}

// GetSecretsForEnvironment returns all Secrets for the specified application and environment.
func GetSecretsForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string, req ...labels.Requirement) ([]corev1.Secret, error) {
	sel := labels.NewSelector().Add(req...)

	ns := operatorutils.GetEnvironmentNamespace(appName, envName)
	secrets, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}

// PatchSecretMetadata sets the updatedAt in a metadata annotation on the secret
func PatchSecretMetadata(secret *corev1.Secret, key string, updatedAt time.Time) error {
	metadata := make(SecretMetadata)
	annotations := secret.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if metadataJson, ok := annotations[RadixMetadataAnnotation]; ok {
		if err := json.Unmarshal([]byte(metadataJson), &metadata); err != nil {
			return err
		}
	}

	metadata[key] = SecretMetadataItem{
		Updated: &updatedAt,
	}

	metadataJsonBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	annotations[RadixMetadataAnnotation] = string(metadataJsonBytes)
	secret.SetAnnotations(annotations)

	return nil
}

// GetSecretMetadata returns a nullsafe SecretMetadata that reads metadata from the secret
func GetSecretMetadata(ctx context.Context, secret *corev1.Secret) *SecretMetadata {
	metadataJson, ok := secret.GetAnnotations()[RadixMetadataAnnotation]
	if !ok {
		log.Ctx(ctx).Debug().Str("namespace", secret.GetNamespace()).Str("secret", secret.GetName()).Msg("No metadata annotation found")
		return nil
	}
	metadata := make(SecretMetadata)
	if err := json.Unmarshal([]byte(metadataJson), &metadata); err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("namespace", secret.GetNamespace()).Str("secret", secret.GetName()).Msg("Failed to unmarshal radix metadata")
		return nil
	}

	return &metadata
}

// GetUpdated reads the updated time from the secret
func (m *SecretMetadata) GetUpdated(key string) *time.Time {
	if m == nil {
		return nil
	}

	item, ok := (*m)[key]
	if !ok {
		return nil
	}

	return item.Updated
}
