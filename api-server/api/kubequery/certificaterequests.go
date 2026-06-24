package kubequery

import (
	"context"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCertificateRequestsForEnvironment returns all CertificateRequests in the environment namespace.
// CertificateRequests are not filtered by app and envionment labels since cert-manager is managing these objects,
// and we have no control of what labels cert-manager will set. All we know is that a CertificateRequest is owned
// by a Certificate, and this is what we use to find correspnding CertificateRequests for a given Certificate.
func GetCertificateRequestsForEnvironment(ctx context.Context, client certclient.Interface, appName, envName string) ([]cmv1.CertificateRequest, error) {
	ns := utils.GetEnvironmentNamespace(appName, envName)
	certReqList, err := client.CertmanagerV1().CertificateRequests(ns).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return certReqList.Items, nil
}
