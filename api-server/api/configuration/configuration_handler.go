package configuration

import (
	"context"

	configurationModels "github.com/equinor/radix-operator/api-server/api/configuration/models"
	"github.com/equinor/radix-operator/pkg/apis/config"
)

type configurationHandler struct {
	cfg config.Config
}

type ConfigurationHandler interface {
	// Init Constructor
	GetClusterConfiguration(ctx context.Context) (configurationModels.ClusterConfiguration, error)
}

// Init Constructor
func Init(config config.Config) ConfigurationHandler {
	return &configurationHandler{
		cfg: config,
	}
}

func (h *configurationHandler) GetClusterConfiguration(ctx context.Context) (configurationModels.ClusterConfiguration, error) {
	return configurationModels.ClusterConfiguration{
		ClusterEgressIps:   h.cfg.ApiServer.ClusterEgressIps,
		ClusterOidcIssuers: h.cfg.ApiServer.ClusterOidcIssuers,
		DNSZone:            h.cfg.Common.DNSZone,
		ClusterName:        h.cfg.Common.ClusterName,
	}, nil
}
