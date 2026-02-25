package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) isOAuth2ProxyModeEnabled() bool {
	return annotations.OAuth2ProxyModeEnabledForEnvironment(deploy.radixDeployment.Annotations, deploy.radixDeployment.Spec.Environment) ||
		annotations.OAuth2ProxyModeEnabledForEnvironment(deploy.registration.Annotations, deploy.radixDeployment.Spec.Environment)
}

func (deploy *Deployment) reconcileIngresses(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	logger := log.Ctx(ctx)
	var hosts []dnsInfo

	// When everyone is using proxy mode, or its enforced, cleanup this code (https://github.com/equinor/radix-platform/issues/1822)
	oauth2enabled := component.GetAuthentication().GetOAuth2() != nil
	oauth2PreviewProxyEnabled := oauth2enabled && deploy.isOAuth2ProxyModeEnabled()
	logger.Debug().Msgf("Reconciling ingresses for component %s. OAuth2 enabled: %t, Proxy mode enabled: %t", component.GetName(), oauth2enabled, oauth2PreviewProxyEnabled)

	if component.IsPublic() && !oauth2PreviewProxyEnabled {
		hosts = getComponentDNSInfo(ctx, component, *deploy.radixDeployment, *deploy.kubeutil)
	}

	if err := deploy.garbageCollectIngresses(ctx, component, hosts); err != nil {
		return fmt.Errorf("failed to garbage collect ingresses: %w", err)
	}

	if err := deploy.createOrUpdateIngress(ctx, component, hosts); err != nil {
		return fmt.Errorf("failed to create ingress: %w", err)
	}

	return nil
}

func (deploy *Deployment) garbageCollectIngresses(ctx context.Context, component radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	logger := log.Ctx(ctx)

	selector := fmt.Sprintf("%s,!%s", labels.ForComponentIngress(deploy.radixDeployment.Spec.AppName, component).AsSelector().String(), kube.RadixAliasLabel)
	existingIngresses, err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("failed to list ingresses in garbageCollectIngresses: %w", err)
	}
	logger.Debug().Msgf("Garbage collecting ingresses for component %s. Found %d existing ingresses", component.GetName(), len(existingIngresses.Items))

	for _, ing := range existingIngresses.Items {
		// should exist in list of hosts
		found := false
		for _, host := range hosts {
			if len(ing.Spec.Rules) > 0 && ing.Spec.Rules[0].Host == host.fqdn {
				found = true
				break
			}
		}

		if !found {
			logger.Info().Msgf("Garbage collecting ingress %s for component %s", ing.Name, component.GetName())
			if err := deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete ingress %s: %w", ing.Name, err)
			}
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateIngress(ctx context.Context, component radixv1.RadixCommonDeployComponent, hosts []dnsInfo) error {
	if len(hosts) == 0 {
		return nil
	}

	owner := []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}

	for _, host := range hosts {
		ingressSpec := ingress.BuildIngressSpecForComponent(component, host.fqdn, host.tlsSecret)
		annotations, err := ingress.BuildAnnotationsFromProviders(component, deploy.ingressAnnotationProviders)
		if err != nil {
			return fmt.Errorf("failed to build annotations: %w", err)
		}
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        host.resourceName,
				Annotations: annotations,
				Labels: labels.Merge(
					labels.ForComponentIngress(deploy.radixDeployment.Spec.AppName, component),
					host.dnsType.ToIngressLabels(),
				),
				OwnerReferences: owner,
			},
			Spec: ingressSpec,
		}

		if err := deploy.kubeutil.ApplyIngress(ctx, deploy.radixDeployment.Namespace, ing); err != nil {
			return fmt.Errorf("failed to reconcile ingress: %w", err)
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectIngressesNoLongerInSpec(ctx context.Context) error {
	ingresses, err := deploy.kubeutil.ListIngresses(ctx, deploy.radixDeployment.Namespace)
	if err != nil {
		return err
	}

	for _, ing := range ingresses {
		componentName, ok := RadixComponentNameFromComponentLabel(ing)
		if !ok {
			continue
		}

		// Ingresses should only exist for items in component list.
		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.NetworkingV1().Ingresses(deploy.radixDeployment.GetNamespace()).Delete(ctx, ing.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
