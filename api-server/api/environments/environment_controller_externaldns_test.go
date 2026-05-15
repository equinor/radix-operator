package environments

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	tlsvalidationmock "github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation/mock"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Test_ExternalDnsTestSuite(t *testing.T) {
	suite.Run(t, new(externalDnsTestSuite))
}

type externalDnsTestSuite struct {
	suite.Suite
	tlsValidator         *tlsvalidationmock.MockValidator
	commonTestUtils      *commontest.Utils
	environmentTestUtils *controllertest.Utils
	kubeClient           kubernetes.Interface
	certClient           *certclientfake.Clientset
	appName              string
	environmentName      string
	alias                string
}

func (s *externalDnsTestSuite) buildCertificate(certCN, issuerCN string, dnsNames []string, notBefore, notAfter time.Time) []byte {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1111),
		Subject:      pkix.Name{CommonName: issuerCN},
		IsCA:         true,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	caPrivKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2222),
		Subject:      pkix.Name{CommonName: certCN},
		DNSNames:     dnsNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	certPrivKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	certBytes, _ := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	certPEM := new(bytes.Buffer)
	err := pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	require.NoError(s.T(), err)
	return certPEM.Bytes()
}

func (s *externalDnsTestSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	s.tlsValidator = tlsvalidationmock.NewMockValidator(ctrl)
	s.commonTestUtils, s.environmentTestUtils, _, s.kubeClient, _, _, _, _, s.certClient = setupTest(s.T(), []EnvironmentHandlerOptions{WithTLSValidator(s.tlsValidator)})

	s.appName, s.environmentName, s.alias = "any-app", "dev", "cdn.myalias.com"
	componentName := "backend"

	_, err := s.commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithAppName(s.appName).
			WithEnvironment(s.environmentName).
			WithComponents(operatorutils.NewDeployComponentBuilder().WithName(componentName).WithExternalDNS(v1.RadixDeployExternalDNS{FQDN: s.alias})).
			WithImageTag("master"))
	require.NoError(s.T(), err)

	_, err = s.commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(s.appName).
		WithEnvironment(s.environmentName, "master").
		WithComponents(operatorutils.
			AnApplicationComponent().
			WithName(componentName)))
	require.NoError(s.T(), err)
}

func (s *externalDnsTestSuite) executeRequest(appName, envName string) (environment *environmentModels.Environment, statusCode int, err error) {
	responseChannel := s.environmentTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, envName))
	response := <-responseChannel
	var env environmentModels.Environment
	err = controllertest.GetResponseBody(response, &env)
	if err == nil {
		environment = &env
	}
	statusCode = response.Code
	return
}

func (s *externalDnsTestSuite) Test_ExternalDNS_Consistent() {
	notBefore, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter, _ := time.Parse("2006-01-02", "2020-08-01")
	certCN, issuerCN := "one.example.com", "issuer.example.com"
	dnsNames := []string{"dns1", "dns2"}
	keyBytes, certBytes := []byte("any key"), s.buildCertificate(certCN, issuerCN, dnsNames, notBefore, notAfter)

	s.tlsValidator.EXPECT().ValidateX509Certificate(certBytes, keyBytes, s.alias).Return(true, nil).Times(1)

	_, err := s.kubeClient.CoreV1().Secrets(s.appName+"-"+s.environmentName).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.alias,
			},
			Data: map[string][]byte{
				corev1.TLSPrivateKeyKey: keyBytes,
				corev1.TLSCertKey:       certBytes,
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(s.T(), err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := []deploymentModels.ExternalDNS{{
		FQDN: s.alias,
		TLS: deploymentModels.TLS{
			Status: deploymentModels.TLSStatusConsistent,
			Certificates: []deploymentModels.X509Certificate{{
				Subject:   "CN=" + certCN,
				Issuer:    "CN=" + issuerCN,
				NotBefore: notBefore,
				NotAfter:  notAfter,
				DNSNames:  dnsNames,
			}},
		},
	}}
	s.ElementsMatch(expectedExternalDNS, environment.ActiveDeployment.Components[0].ExternalDNS)
}

func (s *externalDnsTestSuite) Test_ExternalDNS_MissingPrivateKeyData() {
	notBefore, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter, _ := time.Parse("2006-01-02", "2020-08-01")
	certCN, issuerCN := "one.example.com", "issuer.example.com"
	dnsNames := []string{"dns1", "dns2"}
	certBytes := s.buildCertificate(certCN, issuerCN, dnsNames, notBefore, notAfter)

	_, err := s.kubeClient.CoreV1().Secrets(s.appName+"-"+s.environmentName).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.alias,
			},
			Data: map[string][]byte{
				corev1.TLSPrivateKeyKey: nil,
				corev1.TLSCertKey:       certBytes,
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(s.T(), err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := []deploymentModels.ExternalDNS{{
		FQDN: s.alias,
		TLS: deploymentModels.TLS{
			Status: deploymentModels.TLSStatusPending,
		},
	}}
	s.ElementsMatch(expectedExternalDNS, environment.ActiveDeployment.Components[0].ExternalDNS)
}

func (s *externalDnsTestSuite) Test_ExternalDNS_MissingCertData() {
	keyBytes := []byte("any key")

	_, err := s.kubeClient.CoreV1().Secrets(s.appName+"-"+s.environmentName).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.alias,
			},
			Data: map[string][]byte{
				corev1.TLSPrivateKeyKey: keyBytes,
				corev1.TLSCertKey:       nil,
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(s.T(), err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := []deploymentModels.ExternalDNS{{
		FQDN: s.alias,
		TLS: deploymentModels.TLS{
			Status: deploymentModels.TLSStatusPending,
		},
	}}
	s.ElementsMatch(expectedExternalDNS, environment.ActiveDeployment.Components[0].ExternalDNS)
}

func (s *externalDnsTestSuite) Test_ExternalDNS_CertDataValidationError() {
	notBefore, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter, _ := time.Parse("2006-01-02", "2020-08-01")
	certCN, issuerCN := "one.example.com", "issuer.example.com"
	dnsNames := []string{"dns1", "dns2"}
	keyBytes, certBytes := []byte("any key"), s.buildCertificate(certCN, issuerCN, dnsNames, notBefore, notAfter)
	certValidationMsg := "any msg"

	s.tlsValidator.EXPECT().ValidateX509Certificate(certBytes, keyBytes, s.alias).Return(false, []string{certValidationMsg}).Times(1)

	_, err := s.kubeClient.CoreV1().Secrets(s.appName+"-"+s.environmentName).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.alias,
			},
			Data: map[string][]byte{
				corev1.TLSPrivateKeyKey: keyBytes,
				corev1.TLSCertKey:       certBytes,
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(s.T(), err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := []deploymentModels.ExternalDNS{{
		FQDN: s.alias,
		TLS: deploymentModels.TLS{
			Status:         deploymentModels.TLSStatusInvalid,
			StatusMessages: []string{certValidationMsg},
			Certificates: []deploymentModels.X509Certificate{{
				Subject:   "CN=" + certCN,
				Issuer:    "CN=" + issuerCN,
				NotBefore: notBefore,
				NotAfter:  notAfter,
				DNSNames:  dnsNames,
			}},
		},
	}}
	s.ElementsMatch(expectedExternalDNS, environment.ActiveDeployment.Components[0].ExternalDNS)
}
