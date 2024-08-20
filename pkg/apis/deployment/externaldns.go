package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cm "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	minCertDuration    = 2160 * time.Hour
	minCertRenewBefore = 360 * time.Hour
)

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
		} else {
			if err := deploy.garbageCollectExternalDnsCertificate(ctx, externalDns); err != nil {
				return err
			}

			secretName := utils.GetExternalDnsTlsSecretName(externalDns)
			if err := deploy.createOrUpdateExternalDnsTlsSecret(ctx, externalDns, secretName); err != nil {
				return err
			}
			secretNames = append(secretNames, secretName)
		}
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
	selector := radixlabels.ForApplicationName(deploy.registration.Name).AsSelector()
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

		if err := deploy.deleteSecret(ctx, &secret); err != nil {
			return nil
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectExternalDnsCertificatesNoLongerInSpec(ctx context.Context) error {
	selector := radixlabels.ForApplicationName(deploy.registration.Name).AsSelector()
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
	selector := radixlabels.ForExternalDNSCertificate(deploy.registration.Name, externalDns).AsSelector()
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
	if len(deploy.config.CertificateAutomation.ClusterIssuer) == 0 {
		return errors.New("cluster issuer not set in certificate automation config")
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
			Labels:    radixlabels.ForExternalDNSCertificate(deploy.registration.Name, externalDns),
		},
		Spec: cmv1.CertificateSpec{
			DNSNames: []string{externalDns.FQDN},
			IssuerRef: cmmeta.ObjectReference{
				Group: cm.GroupName,
				Kind:  cmv1.ClusterIssuerKind,
				Name:  deploy.config.CertificateAutomation.ClusterIssuer,
			},
			Duration:    &metav1.Duration{Duration: duration},
			RenewBefore: &metav1.Duration{Duration: renewBefore},
			SecretName:  utils.GetExternalDnsTlsSecretName(externalDns),
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: radixlabels.ForExternalDNSTLSSecret(deploy.registration.Name, externalDns),
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
	secret := v1.Secret{
		Type: v1.SecretTypeTLS,
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: radixlabels.ForExternalDNSTLSSecret(deploy.registration.Name, externalDns),
		},
		Data: tlsSecretDefaultData(),
	}

	existingSecret, err := deploy.kubeclient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		secret.Data = existingSecret.Data
	} else if !k8serrors.IsNotFound(err) {
		return err
	}

	_, err = deploy.kubeutil.ApplySecret(ctx, ns, &secret) //nolint:staticcheck // must be updated to use UpdateSecret or CreateSecret
	if err != nil {
		return err
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
