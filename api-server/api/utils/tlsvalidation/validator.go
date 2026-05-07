package tlsvalidation

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
)

var defaultValidator = validator{}

func DefaultValidator() Validator {
	return &defaultValidator
}

type validator struct{}

func (v *validator) ValidatePrivateKey(keyBytes []byte) (valid bool, failedValidationMessages []string) {
	defer func() {
		valid = len(failedValidationMessages) == 0
	}()

	validatePrivateKey := func(der []byte) error {
		if _, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			return nil
		}
		if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
			switch key.(type) {
			case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
				return nil
			default:
				return errors.New("tls: found unknown private key type in PKCS#8 wrapping")
			}
		}
		if _, err := x509.ParseECPrivateKey(der); err == nil {
			return nil
		}

		return errors.New("tls: failed to parse private key")
	}

	var skippedBlockTypes []string
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyBytes = pem.Decode(keyBytes)
		if keyDERBlock == nil {
			if len(skippedBlockTypes) == 0 {
				failedValidationMessages = append(failedValidationMessages, "tls: failed to find any PEM data in key input")
				return
			}
			failedValidationMessages = append(failedValidationMessages, "tls: failed to find PEM block with type ending in \"PRIVATE KEY\" in key input")
			return
		}
		if keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
		skippedBlockTypes = append(skippedBlockTypes, keyDERBlock.Type)
	}

	if err := validatePrivateKey(keyDERBlock.Bytes); err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	return
}

func (v *validator) ValidateX509Certificate(certBytes, keyBytes []byte, dnsName string) (valid bool, failedValidationMessages []string) {
	defer func() {
		valid = len(failedValidationMessages) == 0
	}()

	certblock, intermediatBytes := pem.Decode(certBytes)
	if certblock == nil || certblock.Type != "CERTIFICATE" {
		failedValidationMessages = append(failedValidationMessages, "x509: missing PEM block for certificate")
		return
	}

	cert, err := x509.ParseCertificate(certblock.Bytes)
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
		return
	}

	_, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM(intermediatBytes)
	_, err = cert.Verify(x509.VerifyOptions{DNSName: dnsName, Intermediates: intermediatePool})
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	return
}
