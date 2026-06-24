package kubequery

import (
	"context"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//

// GetCertificatesForEnvironment returns all Certificates for the specified application and environment.
func GetCertificatesForEnvironment(ctx context.Context, client certclient.Interface, appName, envName string) ([]cmv1.Certificate, error) {
	ns := utils.GetEnvironmentNamespace(appName, envName)
	certList, err := client.CertmanagerV1().Certificates(ns).List(ctx, v1.ListOptions{LabelSelector: labels.ForApplicationName(appName).AsSelector().String()})
	if err != nil {
		return nil, err
	}
	return certList.Items, nil
}
