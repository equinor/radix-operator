package configuration

import (
	"context"

	configurationModels "github.com/equinor/radix-operator/api-server/api/configuration/models"
	"github.com/equinor/radix-operator/api-server/internal/config"
)

type configurationHandler struct {
	config config.Config
}

type ConfigurationHandler interface {
	// Init Constructor
	GetClusterConfiguration(ctx context.Context) (configurationModels.ClusterConfiguration, error)
}

// Init Constructor
func Init(config config.Config) ConfigurationHandler {
	return &configurationHandler{
		config: config,
	}
}

func (h *configurationHandler) GetClusterConfiguration(ctx context.Context) (configurationModels.ClusterConfiguration, error) {
	return configurationModels.ClusterConfiguration{
		ClusterEgressIps:   h.config.ClusterEgressIps,
		ClusterOidcIssuers: h.config.ClusterOidcIssuers,
		DNSZone:            h.config.DNSZone,
		ClusterName:        h.config.ClusterName,
	}, nil
}
