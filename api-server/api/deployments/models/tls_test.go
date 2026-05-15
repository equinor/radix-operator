package models_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func Test_X509CertificateTestSuite(t *testing.T) {
	suite.Run(t, new(x509CertificateTestSuite))
}

type x509CertificateTestSuite struct {
	suite.Suite
}

func (s *x509CertificateTestSuite) Test_ParseX509CertificatesFromPEM_ValidPEM() {
	cn1, ca1, dns1 := "cn1", "ca1", []string{"dns1_1", "dns1_2"}
	notBefore1, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter1, _ := time.Parse("2006-01-02", "2020-08-01")
	cn2, ca2, dns2 := "cn2", "ca2", []string{"dns2_1", "dns2_2"}
	notBefore2, _ := time.Parse("2006-01-02", "2021-07-01")
	notAfter2, _ := time.Parse("2006-01-02", "2021-08-01")

	cert1 := s.buildCert(cn1, ca1, notBefore1, notAfter1, dns1)
	cert2 := s.buildCert(cn2, ca2, notBefore2, notAfter2, dns2)
	b := bytes.NewBuffer(cert1)
	b.Write(cert2)

	expected := []models.X509Certificate{
		{Subject: "CN=" + cn1, Issuer: "CN=" + ca1, NotBefore: notBefore1, NotAfter: notAfter1, DNSNames: dns1},
		{Subject: "CN=" + cn2, Issuer: "CN=" + ca2, NotBefore: notBefore2, NotAfter: notAfter2, DNSNames: dns2},
	}
	certs := models.ParseX509CertificatesFromPEM(b.Bytes())
	s.Equal(expected, certs)
}

func (s *x509CertificateTestSuite) Test_ParseX509CertificatesFromPEM_EmptyPEM() {
	certs := models.ParseX509CertificatesFromPEM(nil)
	s.Empty(certs)
}

func (s *x509CertificateTestSuite) Test_ParseX509CertificatesFromPEM_NonCertificatePEM() {
	cn1, ca1, dns1 := "cn1", "ca1", []string{"dns1_1", "dns1_2"}
	notBefore1, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter1, _ := time.Parse("2006-01-02", "2020-08-01")

	cert1 := s.buildCert(cn1, ca1, notBefore1, notAfter1, dns1)
	cert2 := s.buildCert("anycert", "anyca", time.Now(), time.Now(), nil)
	certBlock, _ := pem.Decode(cert2)
	s.Require().NotNil(certBlock)
	s.Require().Equal("CERTIFICATE", certBlock.Type)
	certBlock.Type = "OTHER"
	var certBuf bytes.Buffer
	err := pem.Encode(&certBuf, certBlock)
	s.Require().NoError(err)
	b := bytes.NewBuffer(cert1)
	b.Write(certBuf.Bytes())

	expected := []models.X509Certificate{
		{Subject: "CN=" + cn1, Issuer: "CN=" + ca1, NotBefore: notBefore1, NotAfter: notAfter1, DNSNames: dns1},
	}
	certs := models.ParseX509CertificatesFromPEM(b.Bytes())
	s.Equal(expected, certs)
}

func (s *x509CertificateTestSuite) Test_ParseX509CertificatesFromPEM_InvalidPEMData() {
	cn1, ca1, dns1 := "cn1", "ca1", []string{"dns1_1", "dns1_2"}
	notBefore1, _ := time.Parse("2006-01-02", "2020-07-01")
	notAfter1, _ := time.Parse("2006-01-02", "2020-08-01")

	cert1 := s.buildCert(cn1, ca1, notBefore1, notAfter1, dns1)
	cert2 := s.buildCert("anycert", "anyca", time.Now(), time.Now(), nil)
	certBlock, _ := pem.Decode(cert2)
	s.Require().NotNil(certBlock)
	s.Require().Equal("CERTIFICATE", certBlock.Type)
	certBlock.Bytes = []byte("invalid data")
	var certBuf bytes.Buffer
	err := pem.Encode(&certBuf, certBlock)
	s.Require().NoError(err)
	b := bytes.NewBuffer(cert1)
	b.Write(certBuf.Bytes())

	expected := []models.X509Certificate{
		{Subject: "CN=" + cn1, Issuer: "CN=" + ca1, NotBefore: notBefore1, NotAfter: notAfter1, DNSNames: dns1},
	}
	certs := models.ParseX509CertificatesFromPEM(b.Bytes())
	s.Equal(expected, certs)
}

func (s *x509CertificateTestSuite) buildCert(certCN, issuerCN string, notBefore, notAfter time.Time, dnsNames []string) []byte {
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
