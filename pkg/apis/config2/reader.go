package config2

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnvConfigMapReader(ctx context.Context, c client.Client) (string, error) {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		log.Ctx(ctx).Warn().Msg("POD_NAMESPACE env var not set, defaulting to 'default' namespace")
		namespace = "default"
	}

	name := os.Getenv("RADIX_COMMON_CONFIG_NAME")
	if name == "" {
		log.Ctx(ctx).Warn().Msg("RADIX_COMMON_CONFIG_NAME env var not set, defaulting to 'radix-common-config' configmap")
		name = "radix-common-config"
	}

	key := os.Getenv("RADIX_COMMON_CONFIG_KEY")
	if key == "" {
		log.Ctx(ctx).Warn().Msg("RADIX_COMMON_CONFIG_KEY env var not set, defaulting to 'configYaml' key")
		key = "configYaml"
	}

	log.Ctx(ctx).Info().Msgf("Loading config from configmap %s/%s.%s", namespace, name, key)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
		return "", fmt.Errorf("failed to load config from cluster: %w", err)
	}

	configYaml, ok := cm.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in configmap %s/%s", key, namespace, name)
	}

	return configYaml, nil
}

func MustEnvConfigMapReader(ctx context.Context, c client.Client) string {
	configYaml, err := EnvConfigMapReader(ctx, c)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read config from configmap")
	}
	return configYaml
}
