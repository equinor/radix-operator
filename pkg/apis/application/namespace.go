package application

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// reconcileAppNamespace creates an app namespace with RadixRegistration as owner
func (app *Application) reconcileAppNamespace(ctx context.Context) error {
	registration := app.registration
	name := utils.GetAppNamespace(registration.Name)

	current, desired, err := app.getCurrentAndDesiredNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current and desired namespace: %w", err)
	}

	if current != nil {
		if appLabel, exists := current.Labels[kube.RadixAppLabel]; !exists || appLabel != registration.Name {
			return fmt.Errorf("namespace %s already exists and is labeled with a missing or different app name: %s", name, appLabel)
		}

		if err := app.kubeutil.UpdateNamespace(ctx, current, desired); err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", name, err)
		}
	} else {
		if _, err := app.kubeutil.CreateNamespace(ctx, desired); err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", name, err)
		}
	}
	log.Ctx(ctx).Info().Msgf("Reconciled namespace %s", name)
	return nil
}

func (app *Application) getCurrentAndDesiredNamespace(ctx context.Context) (current, desired *corev1.Namespace, err error) {

	registration := app.registration
	name := utils.GetAppNamespace(registration.Name)
	currentInternal, err := app.kubeclient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to get namespace %s: %w", name, err)
		}

		desired = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	} else {
		desired = currentInternal.DeepCopy()
		current = currentInternal
	}

	desired.ObjectMeta.OwnerReferences = app.getOwnerReference()
	desired.ObjectMeta.Labels = labels.Merge(desired.ObjectMeta.Labels, map[string]string{
		kube.RadixAppLabel: registration.Name,
		kube.RadixEnvLabel: utils.AppNamespaceEnvName,
	})
	desired.ObjectMeta.Labels = labels.Merge(desired.ObjectMeta.Labels, kube.NewAppNamespacePodSecurityStandardFromEnv().Labels())

	// We don't use snyk anymore, remove line if no more namespaces contains this label
	delete(desired.Labels, "snyk-service-account-sync")

	return current, desired, nil
}
