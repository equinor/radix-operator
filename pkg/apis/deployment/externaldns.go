package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	cm "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/gateway"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapixv1alpha1 "sigs.k8s.io/gateway-api/apisx/v1alpha1"
)

const (
	minCertDuration    = 2160 * time.Hour
	minCertRenewBefore = 360 * time.Hour
)

func (deploy *Deployment) reconcileGatewayResourcesExternalDns(ctx context.Context) error {
	var fqdns []string

	for _, component := range deploy.radixDeployment.Spec.Components {
		if component.IsPublic() == false {
			continue
		}

		for _, ed := range component.GetExternalDNS() {
			fqdns = append(fqdns, ed.FQDN)
			ls, err := deploy.createOrUpdateListenerSetForExternalDns(ctx, ed)
			if err != nil {
				return fmt.Errorf("failed to reconcile ListenerSet for external DNS %s: %w", ed.FQDN, err)
			}
			if err := deploy.createOrUpdateHTTPRouteForExternalDns(ctx, &component, ed, ls); err != nil {
				return fmt.Errorf("failed to reconcile HTTPRoute for external DNS %s: %w", ed.FQDN, err)
			}
		}
	}

	// Garbage collect any ListenerSets and HTTPRoutes for external DNS that are no longer in the spec
	listenersSets := &gatewayapixv1alpha1.XListenerSetList{}
	if err := deploy.dynamicClient.List(ctx, listenersSets, client.InNamespace(deploy.radixDeployment.Namespace)); err != nil {
		return fmt.Errorf("failed to list ListenerSets: %w", err)
	}
	for _, ls := range listenersSets.Items {
		if ls.Labels[kube.RadixAppLabel] != deploy.registration.Name {
			continue
		}

		fqdn, exists := ls.Labels[kube.RadixExternalAliasFQDNLabel]
		if exists && !slices.Contains(fqdns, fqdn) {
			if err := deploy.dynamicClient.Delete(ctx, &ls); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete ListenerSet %s: %w", ls.Name, err)
			}
			log.Ctx(ctx).Info().Str("listenerset", ls.Name).Msg("deleted ListenerSet for external DNS no longer in spec")
		}
	}

	// Garbage collect HTTPRoutes for external DNS that are no longer in the spec
	routes := &gatewayapiv1.HTTPRouteList{}
	if err := deploy.dynamicClient.List(ctx, routes, client.InNamespace(deploy.radixDeployment.Namespace)); err != nil {
		return fmt.Errorf("failed to list HTTPRoutes: %w", err)
	}
	for _, route := range routes.Items {
		if route.Labels[kube.RadixAppLabel] != deploy.registration.Name {
			continue
		}
		fqdn, exist := route.Labels[kube.RadixExternalAliasFQDNLabel]
		if exist && !slices.Contains(fqdns, fqdn) {
			if err := deploy.dynamicClient.Delete(ctx, &route); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete HTTPRoute %s: %w", route.Name, err)
			}
			log.Ctx(ctx).Info().Str("httproute", route.Name).Msg("deleted HTTPRoute for external DNS no longer in spec")
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateHTTPRouteForExternalDns(ctx context.Context, component radixv1.RadixCommonDeployComponent, ed radixv1.RadixDeployExternalDNS, parentListenerSet gatewayapixv1alpha1.XListenerSet) error {
	logger := log.Ctx(ctx)

	oauth2enabled := component.GetAuthentication().GetOAuth2() != nil
	logger.Debug().Msgf("Reconciling HTTPRoute for external dns %s. OAuth2 enabled: %t", ed.FQDN, oauth2enabled)

	route := &gatewayapiv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: ed.FQDN, Namespace: deploy.radixDeployment.Namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, route, func() error {

		lsGVK, err := deploy.dynamicClient.GroupVersionKindFor(&parentListenerSet)
		if err != nil {
			return fmt.Errorf("failed to get GVK for ListenerSet %s: %w", parentListenerSet.Name, err)
		}

		route.Labels = kubelabels.Merge(route.Labels, labels.ForExternalDNSResource(deploy.registration.Name, ed))
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}

		var backendRef gatewayapiv1.HTTPBackendRef
		if oauth2enabled {
			backendRef = gateway.BuildBackendRefForComponentOauth2Service(component)
		} else {
			var err error
			backendRef, err = gateway.BuildBackendRefForComponent(component)
			if err != nil {
				return fmt.Errorf("failed to build backend reference for component %s: %w", component.GetName(), err)
			}
		}

		route.Spec = gatewayapiv1.HTTPRouteSpec{
			Hostnames: []gatewayapiv1.Hostname{
				gatewayapiv1.Hostname(ed.FQDN),
			},
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayapiv1.HTTPBackendRef{backendRef},
					Matches: []gatewayapiv1.HTTPRouteMatch{
						{
							Path: &gatewayapiv1.HTTPPathMatch{Type: new(gatewayapiv1.PathMatchPathPrefix), Value: new("/")},
						},
					},
					Filters: []gatewayapiv1.HTTPRouteFilter{
						{
							Type: gatewayapiv1.HTTPRouteFilterResponseHeaderModifier,
							ResponseHeaderModifier: &gatewayapiv1.HTTPHeaderFilter{
								Add: []gatewayapiv1.HTTPHeader{{Name: "Strict-Transport-Security", Value: "max-age=31536000; includeSubDomains"}},
							},
						},
					},
				},
			},
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{ParentRefs: []gatewayapiv1.ParentReference{
				{
					Group:     new(gatewayapiv1.Group(lsGVK.Group)),
					Kind:      new(gatewayapiv1.Kind(lsGVK.Kind)),
					Name:      gatewayapiv1.ObjectName(parentListenerSet.Name),
					Namespace: new(gatewayapiv1.Namespace(parentListenerSet.Namespace)),
				},
			}},
		}

		route.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		return controllerutil.SetControllerReference(deploy.radixDeployment, route, deploy.dynamicClient.Scheme())
	})

	if err != nil {
		return fmt.Errorf("failed to create or update HTTPRoute '%s': %w", route.Name, err)
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("httproute", route.Name).Str("op", string(op)).Msg("reconcile HTTPRoute")
	}

	return nil
}

func (deploy *Deployment) createOrUpdateListenerSetForExternalDns(ctx context.Context, ed radixv1.RadixDeployExternalDNS) (gatewayapixv1alpha1.XListenerSet, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Reconciling ListenerSet for %s", ed.FQDN)

	ls := gatewayapixv1alpha1.XListenerSet{ObjectMeta: metav1.ObjectMeta{Name: ed.FQDN, Namespace: deploy.radixDeployment.Namespace}}

	op, err := controllerutil.CreateOrUpdate(ctx, deploy.dynamicClient, &ls, func() error {
		ls.Labels = kubelabels.Merge(ls.Labels, labels.ForExternalDNSResource(deploy.registration.Name, ed))
		ls.Spec = gatewayapixv1alpha1.ListenerSetSpec{
			ParentRef: gatewayapixv1alpha1.ParentGatewayReference{
				Group:     new(gatewayapiv1.Group(gatewayapiv1.GroupName)),
				Kind:      new(gatewayapiv1.Kind("Gateway")),
				Name:      gatewayapiv1.ObjectName(deploy.config.Gateway.Name),
				Namespace: new(gatewayapiv1.Namespace(deploy.config.Gateway.Namespace)),
			},
			Listeners: []gatewayapixv1alpha1.ListenerEntry{{
				Name:     gatewayapixv1alpha1.SectionName("https"),
				Hostname: new(gatewayapixv1alpha1.Hostname(ed.FQDN)),
				Protocol: gatewayapiv1.HTTPSProtocolType,
				Port:     443,
				AllowedRoutes: &gatewayapiv1.AllowedRoutes{
					Namespaces: &gatewayapiv1.RouteNamespaces{From: new(gatewayapiv1.NamespacesFromSame)},
				},
				TLS: &gatewayapiv1.ListenerTLSConfig{
					Mode: new(gatewayapiv1.TLSModeTerminate),
					CertificateRefs: []gatewayapiv1.SecretObjectReference{
						{
							Group: new(gatewayapiv1.Group("")),
							Kind:  new(gatewayapiv1.Kind("Secret")),
							Name:  gatewayapiv1.ObjectName(utils.GetExternalDnsTlsSecretName(ed)),
						},
					},
				},
			},
			},
		}

		ls.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
		return controllerutil.SetControllerReference(deploy.radixDeployment, &ls, deploy.dynamicClient.Scheme())
	})

	if err != nil {
		return ls, fmt.Errorf("failed to create or update XListenerSet '%s': %w", ls.Name, err)
	}

	if op != controllerutil.OperationResultNone {
		logger.Info().Str("xlistenerset", ls.Name).Str("op", string(op)).Msg("reconcile XListenerSet")
	}

	return ls, nil
}

func (deploy *Deployment) syncExternalDnsResources(ctx context.Context) error {
	if err := deploy.garbageCollectExternalDnsResourcesNoLongerInSpec(ctx); err != nil {
		return err
	}

	var secretNames []string
	externalDnsList := deploy.getExternalDnsFromAllComponents()

	for _, externalDns := range externalDnsList {
		if externalDns.UseCertificateAutomation {
			if err := deploy.createOrUpdateExternalDnsCertificate(ctx, externalDns); err != nil {
				return err
			}
			continue
		}

		if err := deploy.garbageCollectExternalDnsCertificate(ctx, externalDns); err != nil {
			return err
		}

		secretName := utils.GetExternalDnsTlsSecretName(externalDns)
		if err := deploy.createOrUpdateExternalDnsTlsSecret(ctx, externalDns, secretName); err != nil {
			return err
		}
		secretNames = append(secretNames, secretName)
	}

	return deploy.grantAccessToExternalDnsSecrets(ctx, secretNames)
}

func (deploy *Deployment) garbageCollectExternalDnsResourcesNoLongerInSpec(ctx context.Context) error {
	if err := deploy.garbageCollectExternalDnsCertificatesNoLongerInSpec(ctx); err != nil {
		return err
	}

	return deploy.garbageCollectExternalDnsSecretsNoLongerInSpec(ctx)
}

func (deploy *Deployment) garbageCollectExternalDnsSecretsNoLongerInSpec(ctx context.Context) error {
	selector := labels.ForApplicationName(deploy.registration.Name).AsSelector()
	secrets, err := deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	externalDnsAliases := deploy.getExternalDnsFromAllComponents()
	for _, secret := range secrets.Items {
		fqdn, ok := secret.Labels[kube.RadixExternalAliasFQDNLabel]
		if !ok {
			continue
		}

		if slice.Any(externalDnsAliases, func(rded radixv1.RadixDeployExternalDNS) bool { return rded.FQDN == fqdn }) {
			continue
		}
		if err := deploy.kubeutil.DeleteSecret(ctx, deploy.radixDeployment.Namespace, secret.Name); err != nil {
			return nil
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectExternalDnsCertificatesNoLongerInSpec(ctx context.Context) error {
	selector := labels.ForApplicationName(deploy.registration.Name).AsSelector()
	certificates, err := deploy.certClient.CertmanagerV1().Certificates(deploy.radixDeployment.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	externalDnsAliases := deploy.getExternalDnsFromAllComponents()
	for _, cert := range certificates.Items {
		fqdn, ok := cert.Labels[kube.RadixExternalAliasFQDNLabel]
		if !ok {
			continue
		}

		if slice.Any(externalDnsAliases, func(rded radixv1.RadixDeployExternalDNS) bool { return rded.FQDN == fqdn }) {
			continue
		}

		if err := deploy.certClient.CertmanagerV1().Certificates(cert.Namespace).Delete(ctx, cert.Name, metav1.DeleteOptions{}); err != nil {
			return nil
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectExternalDnsCertificate(ctx context.Context, externalDns radixv1.RadixDeployExternalDNS) error {
	selector := labels.ForExternalDNSResource(deploy.registration.Name, externalDns).AsSelector()
	certs, err := deploy.certClient.CertmanagerV1().Certificates(deploy.radixDeployment.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	for _, cert := range certs.Items {
		if err := deploy.certClient.CertmanagerV1().Certificates(cert.Namespace).Delete(ctx, cert.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateExternalDnsCertificate(ctx context.Context, externalDns radixv1.RadixDeployExternalDNS) error {
	if len(deploy.config.CertificateAutomation.GatewayClusterIssuer) == 0 {
		return errors.New("gateway cluster issuer not set in certificate automation config")
	}

	duration := deploy.config.CertificateAutomation.Duration
	if duration < minCertDuration {
		duration = minCertDuration
	}
	renewBefore := deploy.config.CertificateAutomation.RenewBefore
	if renewBefore < minCertRenewBefore {
		renewBefore = minCertRenewBefore
	}

	certificate := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      externalDns.FQDN,
			Namespace: deploy.radixDeployment.Namespace,
			Labels:    labels.ForExternalDNSResource(deploy.registration.Name, externalDns),
		},
		Spec: cmv1.CertificateSpec{
			DNSNames: []string{externalDns.FQDN},
			IssuerRef: cmmeta.ObjectReference{
				Group: cm.GroupName,
				Kind:  cmv1.ClusterIssuerKind,
				Name:  deploy.config.CertificateAutomation.GatewayClusterIssuer,
			},
			Duration:    &metav1.Duration{Duration: duration},
			RenewBefore: &metav1.Duration{Duration: renewBefore},
			SecretName:  utils.GetExternalDnsTlsSecretName(externalDns),
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: labels.ForExternalDNSResource(deploy.registration.Name, externalDns),
			},
			PrivateKey: &cmv1.CertificatePrivateKey{
				RotationPolicy: cmv1.RotationPolicyAlways,
			},
		},
	}

	return deploy.applyCertificate(ctx, certificate)
}

func (deploy *Deployment) applyCertificate(ctx context.Context, cert *cmv1.Certificate) error {
	existingCert, err := deploy.certClient.CertmanagerV1().Certificates(cert.Namespace).Get(ctx, cert.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = deploy.certClient.CertmanagerV1().Certificates(cert.Namespace).Create(ctx, cert, metav1.CreateOptions{})
			return err
		}
		return err
	}

	newCert := existingCert.DeepCopy()
	newCert.ObjectMeta.Labels = cert.ObjectMeta.Labels
	newCert.ObjectMeta.Annotations = cert.ObjectMeta.Annotations
	newCert.ObjectMeta.OwnerReferences = cert.ObjectMeta.OwnerReferences
	newCert.Spec = cert.Spec

	exitingCertBytes, err := json.Marshal(existingCert)
	if err != nil {
		return fmt.Errorf("failed to marshal existing certificate %s/%s: %w", existingCert.Namespace, existingCert.Name, err)
	}

	newCertBytes, err := json.Marshal(newCert)
	if err != nil {
		return fmt.Errorf("failed to marshal new certificate %s/%s: %w", newCert.Namespace, newCert.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(exitingCertBytes, newCertBytes, &cmv1.Certificate{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch for certificate %s/%s: %w", newCert.Namespace, newCert.Name, err)
	}

	if !kube.IsEmptyPatch(patchBytes) {
		_, err = deploy.certClient.CertmanagerV1().Certificates(newCert.Namespace).Update(ctx, newCert, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update certificate %s/%s: %w", newCert.Namespace, newCert.Name, err)
		}
	}

	return nil
}

func (deploy *Deployment) createOrUpdateExternalDnsTlsSecret(ctx context.Context, externalDns radixv1.RadixDeployExternalDNS, secretName string) error {
	ns := deploy.radixDeployment.Namespace
	var currentSecret, desiredSecret *corev1.Secret
	currentSecret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get current external DNS secret: %w", err)
		}
		desiredSecret = &corev1.Secret{
			Type: corev1.SecretTypeTLS,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       nil,
				corev1.TLSPrivateKeyKey: nil,
			},
		}
		currentSecret = nil
	} else {
		desiredSecret = currentSecret.DeepCopy()
	}

	desiredSecret.Labels = labels.ForExternalDNSResource(deploy.registration.Name, externalDns)

	if currentSecret == nil {
		if _, err := deploy.kubeutil.CreateSecret(ctx, ns, desiredSecret); err != nil {
			return fmt.Errorf("failed to create external DNS secret: %w", err)
		}
		return nil
	}

	if _, err := deploy.kubeutil.UpdateSecret(ctx, currentSecret, desiredSecret); err != nil {
		return fmt.Errorf("failed to update external DNS secret: %w", err)
	}

	return nil
}

func (deploy *Deployment) getExternalDnsFromAllComponents() []radixv1.RadixDeployExternalDNS {
	var externalDns []radixv1.RadixDeployExternalDNS

	for _, comp := range deploy.radixDeployment.Spec.Components {
		externalDns = append(externalDns, comp.GetExternalDNS()...)
	}

	return externalDns
}
