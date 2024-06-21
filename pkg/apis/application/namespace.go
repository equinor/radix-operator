package application

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/labels"
)

// createAppNamespace creates an app namespace with RadixRegistration as owner
func (app *Application) createAppNamespace(ctx context.Context) error {
	registration := app.registration
	name := utils.GetAppNamespace(registration.Name)

	nsLabels := map[string]string{
		kube.RadixAppLabel:          registration.Name,
		kube.RadixEnvLabel:          utils.AppNamespaceEnvName,
		"snyk-service-account-sync": "radix-snyk-service-account",
	}
	nsLabels = labels.Merge(nsLabels, kube.NewAppNamespacePodSecurityStandardFromEnv().Labels())

	ownerRef := app.getOwnerReference()
	err := app.kubeutil.ApplyNamespace(ctx, name, nsLabels, ownerRef)

	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	log.Ctx(ctx).Info().Msgf("Created namespace %s", name)
	return nil
}
