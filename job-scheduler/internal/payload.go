package internal

import (
	"context"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
)

// GarbageCollectPayloadSecrets Delete orphaned payload secrets
func GarbageCollectPayloadSecrets(ctx context.Context, kubeUtil *kube.Kube, namespace, radixComponentName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Garbage collecting payload secrets")
	payloadSecretRefNames, err := getJobComponentPayloadSecretRefNames(ctx, kubeUtil.RadixClient(), namespace, radixComponentName)
	if err != nil {
		return err
	}
	payloadSecrets, err := kubeUtil.ListSecretsWithSelector(ctx, namespace, labels.GetRadixBatchDescendantsSelector(radixComponentName).String())
	if err != nil {
		return err
	}
	logger.Debug().Msgf("%d payload secrets, %d secret reference unique names", len(payloadSecrets), len(payloadSecretRefNames))
	yesterday := time.Now().Add(time.Hour * -24)
	for _, payloadSecret := range payloadSecrets {
		if _, ok := payloadSecretRefNames[payloadSecret.GetName()]; !ok {
			if payloadSecret.GetCreationTimestamp().After(yesterday) {
				logger.Debug().Msgf("skipping deletion of an orphaned payload secret %s, created within 24 hours", payloadSecret.GetName())
				continue
			}
			if err := kubeUtil.DeleteSecret(ctx, payloadSecret.GetNamespace(), payloadSecret.GetName()); err != nil && !errors.IsNotFound(err) {
				logger.Error().Err(err).Msgf("failed deleting of an orphaned payload secret %s", payloadSecret.GetName())
			} else {
				logger.Debug().Msgf("deleted an orphaned payload secret %s", payloadSecret.GetName())
			}
		}
	}
	return nil
}

func getJobComponentPayloadSecretRefNames(ctx context.Context, radixClient radixclient.Interface, namespace, radixComponentName string) (map[string]bool, error) {
	radixBatches, err := GetRadixBatches(ctx, radixClient, namespace, labels.ForComponentName(radixComponentName))
	if err != nil {
		return nil, err
	}
	payloadSecretRefNames := make(map[string]bool)
	for _, radixBatch := range radixBatches {
		for _, job := range radixBatch.Spec.Jobs {
			if job.PayloadSecretRef != nil {
				payloadSecretRefNames[job.PayloadSecretRef.Name] = true
			}
		}
	}
	return payloadSecretRefNames, nil
}
