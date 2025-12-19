package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/oauth"
	"github.com/rs/zerolog/log"
	networkingv1 "k8s.io/api/networking/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// CreateRadixDNSAliasIngress Create an Ingress for a RadixDNSAlias
func CreateRadixDNSAliasIngress(ctx context.Context, kubeClient kubernetes.Interface, appName, envName string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return kubeClient.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace(appName, envName)).Create(ctx, ingress, metav1.CreateOptions{})
}

func (s *syncer) syncIngresses(ctx context.Context) error {
	/*
		syncIngresses:
			component, err:=getComponentForActiveDeployment()
			if err!=nil {
				return err
			}

			// Hvordan skal vi håndtere at component ikke finnes (no longer in spec)?
			if component==nil {
				deleteAllIngresses(component)
				return // Bør vi på en eller annen måte skrive i RDA.Status.Message at deployment eller component ikke finnes?
			}

			garbageCollectOAuthIngress(component)
			garbageCollectCollectComponentIngress(component)

			createOrUpdateComponentIngress(component)
			createOrUpdateOAuthIngress(component)

		deleteAllIngresses:
			ings=list ingresses with selector
			delete all ings

		garbageCollectOAuthIngress:
			ing:=get ing with label selector
			if not found {
				return
			}

			delete=func() {}
				if !comp.IsPublic() {
					return true
				}
				if !comp.OAuth() {
					return true
				}

				comp=buildComponentWithOAuthDefaults(comp)
				path=ingress.BuildIngressSpecForOAuth2Component(comp, isProxyMode()).Path

				if ing.Path!=path {
					return true
				}

				return false
			}

			if !delete() {

			}

			kube delete ing

		garbageCollectCollectComponentIngress:
			ing:=get ing with label selector
			if not found {
				return
			}

			delete=func() {
				if !comp.IsPublic() {
					return true
				}

				return comp.OAuth() && isProxyMode()
			}

			if !delete() {
				return
			}

			kube delete ing

		createOrUpdateComponentIngress:
			apply=func() {
				if !comp.IsPublic() {
					return false
				}

				return !comp.OAuth() || !isProxyMode() {
			}

			if !apply() {
				return
			}

			ing:=build Ingress with Spec
			ApplyIngress ing

		createOrUpdateOAuthIngress:
			apply=func() {
				if !comp.IsPublic() {
					return false
				}

				return comp.OAuth()
			}

			if !apply() {
				return
			}

			comp=buildComponentWithOAuthDefaults(comp)
			spec=ingress.BuildIngressSpecForOAuth2Component()
			annotations=select based on isProxyMode()
			ing=spec+annotations
			ApplyIngress ing


	*/

	// if s.component == nil {
	// 	log.Ctx(ctx).Info().Msg("Component not found, deleting existing ingresses.")
	// 	return s.deleteIngresses(ctx)
	// }

	if err := s.garbageCollectOAuthIngress(ctx); err != nil {
		return err
	}

	if err := s.garbageCollectComponentIngress(ctx); err != nil {
		return err
	}

	if err := s.createOrUpdateComponentIngress(ctx); err != nil {
		return err
	}

	if err := s.createOrUpdateOAuthIngress(ctx); err != nil {
		return err
	}

	return nil
}

func (s *syncer) createOrUpdateComponentIngress(ctx context.Context) error {
	mustApply := func() bool {
		if s.component == nil {
			return false
		}

		if !s.component.IsPublic() {
			return false
		}

		if s.component.GetAuthentication().GetOAuth2() != nil && s.isProxyModeEnabled() {
			return false
		}

		return true
	}

	if !mustApply() {
		return nil
	}

	annotations, err := ingress.BuildAnnotationsFromProviders(s.component, s.componentIngressAnnotations)
	if err != nil {
		return fmt.Errorf("failed to build annotations for component ingress: %w", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.getIngressName(),
			Labels:      radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias),
			Annotations: annotations,
		},
		Spec: ingress.BuildIngressSpecForComponent(s.component, s.getHostName(), ""),
	}

	if err := s.kubeUtil.ApplyIngress(ctx, "", ing); err != nil {
		return fmt.Errorf("failed to apply component ingress: %w", err)
	}

	return nil
}

func (s *syncer) createOrUpdateOAuthIngress(ctx context.Context) error {
	/*
		createOrUpdateOAuthIngress:
			apply=func() {
				if comp==nil {return false}
				if !comp.IsPublic() {
					return false
				}

				return comp.OAuth()
			}

			if !apply() {
				return
			}

			comp=buildComponentWithOAuthDefaults(comp)
			spec=ingress.BuildIngressSpecForOAuth2Component()
			annotations=select based on isProxyMode()
			ing=spec+annotations
			ApplyIngress ing with Spec
			ApplyIngress ing
	*/

	mustApply := func() bool {
		if s.component == nil {
			return false
		}

		if !s.component.IsPublic() {
			return false
		}

		return s.component.GetAuthentication().GetOAuth2() != nil
	}

	if !mustApply() {
		return nil
	}

	annotationProviders := s.oauthIngressAnnotations
	if s.isProxyModeEnabled() {
		annotationProviders = s.oauthProxyModeIngressAnnotation
	}
	annotations, err := ingress.BuildAnnotationsFromProviders(s.component, annotationProviders)
	if err != nil {
		return fmt.Errorf("failed to build annotations for component ingress: %w", err)
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        oauth.GetAuxOAuthProxyIngressName(s.getIngressName()),
			Labels:      radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias),
			Annotations: annotations,
		},
		Spec: ingress.BuildIngressSpecForOAuth2Component(s.component, s.getHostName(), "", s.isProxyModeEnabled()),
	}

	if err := s.kubeUtil.ApplyIngress(ctx, "", ing); err != nil {
		return fmt.Errorf("failed to apply oauth ingress: %w", err)
	}

	return nil
}

func (s *syncer) garbageCollectOAuthIngress(ctx context.Context) error {
	ingressName := oauth.GetAuxOAuthProxyIngressName(s.getIngressName())
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)

	ing, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get oauth ingress for garbage collect: %w", err)
	}

	mustDelete := func() bool {
		if s.component == nil {
			return true
		}

		if !s.component.IsPublic() {
			return true
		}

		if s.component.GetAuthentication().GetOAuth2() == nil {
			return true
		}

		// expectedLabels := radixlabels.ForDNSAliasOAuthIngress(s.radixDNSAlias)
		// if !expectedLabels.AsSelector().Matches(labels.Set(ing.Labels)) {
		// 	return true
		// }

		expectedPath := ingress.BuildIngressSpecForOAuth2Component(s.component, "", "", s.isProxyModeEnabled()).Rules[0].HTTP.Paths[0].Path

		return ing.Spec.Rules[0].HTTP.Paths[0].Path != expectedPath
	}

	if !mustDelete() {
		return nil
	}

	if err := s.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete oauth ingress: %w", err)
	}

	return nil
}

func (s *syncer) garbageCollectComponentIngress(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)
	ing, err := s.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, s.getIngressName(), metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get ingress for garbage collect: %w", err)
	}

	mustDelete := func() bool {
		if s.component == nil {
			return true
		}

		if !s.component.IsPublic() {
			return true
		}

		// expectedLabels := radixlabels.ForDNSAliasComponentIngress(s.radixDNSAlias)
		// if !expectedLabels.AsSelector().Matches(labels.Set(ing.Labels)) {
		// 	return true
		// }

		return s.component.GetAuthentication().GetOAuth2() != nil && s.isProxyModeEnabled()
	}

	if !mustDelete() {
		return nil
	}

	if err := s.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete ingress: %w", err)
	}

	return nil
}

func (s *syncer) deleteIngresses(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.Spec.Environment)

	ingresses, err := s.kubeClient.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: kube.RadixAliasLabel})
	if err != nil {
		return err
	}

	for _, ing := range ingresses.Items {
		log.Ctx(ctx).Info().Msgf("Deleting ingress %s", cache.MetaObjectToName(&ing).String())

		if err := s.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete ingress: %w", err)
		}
	}

	return nil
}

func (s *syncer) buildComponentWithOAuthDefaults(component *radixv1.RadixDeployComponent) (*radixv1.RadixDeployComponent, error) {
	if component.GetAuthentication().GetOAuth2() == nil {
		return component, nil
	}
	componentWithOAuthDefaults := component.DeepCopy()
	oauth, err := s.oauth2DefaultConfig.MergeWith(componentWithOAuthDefaults.Authentication.OAuth2)
	if err != nil {
		return nil, err
	}
	componentWithOAuthDefaults.Authentication.OAuth2 = oauth
	return componentWithOAuthDefaults, nil
}

func (s *syncer) isProxyModeEnabled() bool {
	var rdProxyMode, rrProxyMode bool

	if s.rd != nil {
		rdProxyMode = annotations.OAuth2ProxyModeEnabledForEnvironment(s.rd.Annotations, s.rd.Spec.Environment)
	}

	rrProxyMode = annotations.OAuth2ProxyModeEnabledForEnvironment(s.rr.Annotations, s.rd.Spec.Environment)

	return rdProxyMode || rrProxyMode
}

func (s *syncer) getHostName() string {
	return fmt.Sprintf("%s.%s", s.radixDNSAlias.Name, s.dnsZone)
}

func (s *syncer) getIngressName() string {
	return fmt.Sprintf("%s.custom-alias", s.radixDNSAlias.Name)
}

// func (s *syncer) syncIngress(ctx context.Context, namespace string, radixDeployComponent radixv1.RadixCommonDeployComponent) (*networkingv1.Ingress, error) {
// 	ingressName := getDNSAliasIngressName(s.radixDNSAlias.GetName())
// 	newIngress, err := buildIngress(radixDeployComponent, s.radixDNSAlias, s.dnsZone, s.oauth2DefaultConfig, s.ingressConfiguration)
// 	if err != nil {
// 		return nil, err
// 	}
// 	existingIngress, err := s.kubeUtil.GetIngress(ctx, namespace, ingressName)
// 	if err != nil {
// 		if kubeerrors.IsNotFound(err) {
// 			log.Ctx(ctx).Debug().Msgf("not found Ingress %s in the namespace %s. Create new.", ingressName, namespace)
// 			return s.createIngress(ctx, s.radixDNSAlias, newIngress)
// 		}
// 		return nil, err
// 	}
// 	log.Ctx(ctx).Debug().Msgf("found Ingress %s in the namespace %s.", ingressName, namespace)
// 	patchesIngress, err := s.applyIngress(ctx, namespace, existingIngress, newIngress)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return patchesIngress, nil
// }

// func (s *syncer) applyIngress(ctx context.Context, namespace string, existingIngress *networkingv1.Ingress, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
// 	patchesIngress, err := s.kubeUtil.PatchIngress(ctx, namespace, existingIngress, ing)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to patch an ingress %s: %w", ing.GetName(), err)
// 	}
// 	return patchesIngress, nil
// }

// func (s *syncer) createIngress(ctx context.Context, radixDNSAlias *radixv1.RadixDNSAlias, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
// 	log.Ctx(ctx).Debug().Msgf("create an ingress %s for the RadixDNSAlias", ing.GetName())
// 	return CreateRadixDNSAliasIngress(ctx, s.kubeClient, radixDNSAlias.Spec.AppName, radixDNSAlias.Spec.Environment, ing)
// }

// func (s *syncer) syncOAuthProxyIngress(ctx context.Context, namespace string, ing *networkingv1.Ingress, deployComponent radixv1.RadixCommonDeployComponent) error {
// 	appName := s.radixDNSAlias.Spec.AppName
// 	authentication := deployComponent.GetAuthentication()
// 	oauthEnabled := authentication != nil && authentication.OAuth2 != nil
// 	if !oauthEnabled {
// 		selector := radixlabels.ForDNSAliasOAuthIngress(s.radixDNSAlias.Spec.AppName, deployComponent, s.radixDNSAlias.GetName())
// 		return s.deleteIngresses(ctx, selector)
// 	}
// 	oauth2, err := s.oauth2DefaultConfig.MergeWith(authentication.OAuth2)
// 	if err != nil {
// 		return err
// 	}
// 	authentication.OAuth2 = oauth2
// 	auxIngress, err := ingress.BuildOAuthProxyIngressForComponentIngress(namespace, deployComponent, ing, s.ingressAnnotationProviders)
// 	if err != nil {
// 		return err
// 	}
// 	if auxIngress == nil {
// 		return nil
// 	}
// 	oauth.MergeAuxComponentDNSAliasIngressResourceLabels(auxIngress, appName, deployComponent, s.radixDNSAlias.GetName())
// 	return s.kubeUtil.ApplyIngress(ctx, namespace, auxIngress)
// }

// func buildIngress(radixDeployComponent radixv1.RadixCommonDeployComponent, radixDNSAlias *radixv1.RadixDNSAlias, dnsZone string, oauth2Config defaults.OAuth2Config, ingressConfiguration ingress.IngressConfiguration) (*networkingv1.Ingress, error) {
// 	if !radixDeployComponent.IsPublic() {
// 		return nil, ErrComponentIsNotPublic
// 	}

// 	aliasName := radixDNSAlias.GetName()
// 	aliasSpec := radixDNSAlias.Spec
// 	ingressName := GetDNSAliasIngressName(aliasName)
// 	hostName := GetDNSAliasHost(aliasName, dnsZone)
// 	ingressSpec := ingress.BuildIngressSpecForComponent(radixDeployComponent, hostName, "")

// 	namespace := utils.GetEnvironmentNamespace(aliasSpec.AppName, aliasSpec.Environment)
// 	ingressAnnotations := ingress.GetComponentAnnotationProvider(ingressConfiguration, namespace, oauth2Config)
// 	ingressConfig, err := ingress.BuildIngress(aliasSpec.AppName, radixDeployComponent, ingressName, ingressSpec, ingressAnnotations, internal.GetOwnerReferences(radixDNSAlias, true))
// 	if err != nil {
// 		return nil, err
// 	}

// 	ingressConfig.ObjectMeta.Labels[kube.RadixAliasLabel] = aliasName
// 	return ingressConfig, nil
// }
