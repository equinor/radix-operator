package config2

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

func EnvConfigMapReader(c client.Client) ConfigReader {
	return func(ctx context.Context) (string, error) {
		logger := log.Ctx(ctx).With().Str("pkg", "config2").Logger()
		
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			logger.Warn().Msg("POD_NAMESPACE env var not set, defaulting to 'default' namespace")
			namespace = "default"
		}

		name := os.Getenv("RADIX_OPERATOR_CONFIG_NAME")
		if name == "" {
			logger.Warn().Msg("RADIX_OPERATOR_CONFIG_NAME env var not set, defaulting to 'radix-common-config' configmap")
			name = "radix-common-config"
		}

		key := os.Getenv("RADIX_OPERATOR_CONFIG_KEY")
		if key == "" {
			logger.Warn().Msg("RADIX_OPERATOR_CONFIG_KEY env var not set, defaulting to 'configYaml' key")
			key = "configYaml"
		}

		logger.Info().Msgf("Loading config from configmap %s/%s.%s", namespace, name, key)
		cm := &corev1.ConfigMap{Namespace: namespace, Name: name}
		if err := c.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
			return "", fmt.Errorf("failed to load config from cluster: %w", err)
		}

		// Parse config
		configYaml := cm.Data[key]
		return configYaml, nil
	}
}
