package models

import (
	"crypto/x509"
	"encoding/pem"
	"time"
)

// swagger:enum TLSStatusEnum
type TLSStatusEnum string

const (
	// TLS certificate and private key not set
	TLSStatusPending TLSStatusEnum = "Pending"
	// TLS certificate and private key is valid
	TLSStatusConsistent TLSStatusEnum = "Consistent"
	// TLS certificate and private key is invalid
	TLSStatusInvalid TLSStatusEnum = "Invalid"
)

// TLS configuration and status for external DNS
// swagger:model TLS
type TLS struct {
	// UseAutomation describes if TLS certificate is automatically issued using automation (ACME)
	//
	// required: true
	UseAutomation bool `json:"useAutomation"`

	// Automation describes the current condition of a certificate automation request
	// Only set if UseAutomation is true
	//
	// required: false
	Automation *TLSAutomation `json:"automation,omitempty"`

	// Status of TLS certificate and private key
	//
	// required: true
	// example: Consistent
	Status TLSStatusEnum `json:"status"`

	// StatusMessages contains a list of messages related to Status
	//
	// required: false
	StatusMessages []string `json:"statusMessages,omitempty"`

	// Certificates holds the X509 certificate chain
	// The first certificate in the list should be the host certificate and the rest should be intermediate certificates
	//
	// required: false
	Certificates []X509Certificate `json:"certificates,omitempty"`
}

// swagger:enum TLSAutomationStatusEnum
type TLSAutomationStatusEnum string

const (
	// Certificate automation request pending
	TLSAutomationPending TLSAutomationStatusEnum = "Pending"

	// Certificate automation request succeeded
	TLSAutomationSuccess TLSAutomationStatusEnum = "Success"

	// Certificate automation request failed
	TLSAutomationFailed TLSAutomationStatusEnum = "Failed"
)

// TLSAutomation describes the current condition of TLS automation
// swagger:model TLSAutomation
type TLSAutomation struct {
	// Status of certificate automation request
	//
	// required: true
	// example: Pending
	Status TLSAutomationStatusEnum `json:"status"`

	// Message is a human readable description of the reason for the status
	//
	// required: false
	Message string `json:"message"`
}

// X509Certificate holds information about a X509 certificate
// swagger:model X509Certificate
type X509Certificate struct {
	// Subject contains the distinguished name for the certificate
	//
	// required: true
	// example: CN=mysite.example.com,O=MyOrg,L=MyLocation,C=NO
	Subject string `json:"subject"`
	// Issuer contains the distinguished name for the certificate's issuer
	//
	// required: true
	// example: CN=DigiCert TLS RSA SHA256 2020 CA1,O=DigiCert Inc,C=US
	Issuer string `json:"issuer"`
	// NotBefore defines the lower date/time validity boundary
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2022-08-09T00:00:00Z
	NotBefore time.Time `json:"notBefore"`
	// NotAfter defines the uppdater date/time validity boundary
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2023-08-25T23:59:59Z
	NotAfter time.Time `json:"notAfter"`
	// DNSNames defines list of Subject Alternate Names in the certificate
	//
	// required: false
	DNSNames []string `json:"dnsNames,omitempty"`
}

// ParseX509CertificatesFromPEM builds an array of X509Certificate from PEM encoded data
func ParseX509CertificatesFromPEM(certBytes []byte) []X509Certificate {
	var certs []X509Certificate
	for len(certBytes) > 0 {
		var block *pem.Block
		block, certBytes = pem.Decode(certBytes)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		certs = append(certs, X509Certificate{
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
			DNSNames:  cert.DNSNames,
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
		})
	}

	return certs
}
