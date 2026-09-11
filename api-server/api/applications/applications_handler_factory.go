package applications

import (
	"context"

	"github.com/equinor/radix-operator/api-server/api/utils/access"
	"github.com/equinor/radix-operator/api-server/internal/accounts"
	"github.com/equinor/radix-operator/pkg/apis/config"
	authorizationapi "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
)

// ApplicationHandlerFactory defines a factory function for creating an ApplicationHandler
type ApplicationHandlerFactory interface {
	Create(accounts accounts.Accounts) ApplicationHandler
}

type applicationHandlerFactory struct {
	cfg config.Config
}

// NewApplicationHandlerFactory creates a new ApplicationHandlerFactory
func NewApplicationHandlerFactory(config config.Config) ApplicationHandlerFactory {
	return &applicationHandlerFactory{
		cfg: config,
	}
}

// Create creates a new ApplicationHandler
func (f *applicationHandlerFactory) Create(accounts accounts.Accounts) ApplicationHandler {
	return NewApplicationHandler(accounts, f.cfg, hasAccessToGetConfigMap)
}

func hasAccessToGetConfigMap(ctx context.Context, kubeClient kubernetes.Interface, namespace, configMapName string) (bool, error) {
	return access.HasAccess(ctx, kubeClient, &authorizationapi.ResourceAttributes{
		Verb:      "get",
		Group:     "",
		Resource:  "configmaps",
		Version:   "*",
		Namespace: namespace,
		Name:      configMapName,
	})
}
