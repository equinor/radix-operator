// Package dex manages EasyAuth OAuth2 clients in Dex through the Dex gRPC API
package dex

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	api "github.com/dexidp/dex/api/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ClientIDPrefix identifies Dex clients owned by EasyAuth, since Dex clients have no labels or metadata
const ClientIDPrefix = "easyauth."

// ClientID returns the Dex client ID of an application, see easyauth/README.md
func ClientID(appID string) string {
	return ClientIDPrefix + appID
}

// ClientIDForApp returns the Dex client ID of an application, from the AppID of its RadixRegistration
func ClientIDForApp(ctx context.Context, c client.Reader, appName string) (string, error) {
	rr := &radixv1.RadixRegistration{}
	if err := c.Get(ctx, client.ObjectKey{Name: appName}, rr); err != nil {
		return "", fmt.Errorf("failed to get RadixRegistration %s: %w", appName, err)
	}
	if rr.Spec.AppID.IsZero() {
		return "", fmt.Errorf("RadixRegistration %s has no appID", appName)
	}
	return ClientID(rr.Spec.AppID.String()), nil
}

// RedirectURI returns the OAuth2 proxy callback URL for a public host
func RedirectURI(host string) string {
	return fmt.Sprintf("https://%s/oauth2/callback", host)
}

// NewClient returns a client for the Dex gRPC API at address, authenticated with the client certificate in certsDir.
// certsDir must contain tls.crt, tls.key and ca.crt, as written by cert-manager. The client certificate is read on
// every TLS handshake, so a certificate renewed by cert-manager is used without a restart.
func NewClient(address, certsDir string) (api.DexClient, *grpc.ClientConn, error) {
	caPEM, err := os.ReadFile(filepath.Join(certsDir, "ca.crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Dex CA certificate: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, nil, errors.New("failed to parse Dex CA certificate")
	}
	certFile, keyFile := filepath.Join(certsDir, "tls.crt"), filepath.Join(certsDir, "tls.key")
	// Fail early if the client certificate is missing or invalid
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return nil, nil, fmt.Errorf("failed to load Dex client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load Dex client certificate: %w", err)
			}
			return &cert, nil
		},
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Dex gRPC client: %w", err)
	}
	return api.NewDexClient(conn), conn, nil
}

// IsNotFound returns true if err is returned by the Dex API for a client that doesn't exist.
// GetClient returns the storage error as is, which gRPC sends as codes.Unknown with the message "not found".
func IsNotFound(err error) bool {
	s, ok := status.FromError(err)
	if !ok || err == nil {
		return false
	}
	return s.Code() == codes.NotFound || (s.Code() == codes.Unknown && s.Message() == "not found")
}

// EnsureClient creates the client if it doesn't exist, and returns the client secret stored in Dex.
// An existing client keeps its secret and redirect URIs. Missing redirect URIs are added and the name is updated,
// since other clusters sharing the Dex database may use other redirect URIs for the same client.
func EnsureClient(ctx context.Context, dex api.DexClient, id, name string, redirectURIs []string) (string, error) {
	resp, err := dex.GetClient(ctx, &api.GetClientReq{Id: id})
	if IsNotFound(err) {
		created, err := createClient(ctx, dex, id, name, redirectURIs)
		if err != nil {
			return "", err
		}
		if created != nil {
			return created.GetSecret(), nil
		}
		// Created concurrently, for example by another cluster
		resp, err = dex.GetClient(ctx, &api.GetClientReq{Id: id})
		if err != nil {
			return "", fmt.Errorf("failed to get Dex client %s: %w", id, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to get Dex client %s: %w", id, err)
	}

	existing := resp.GetClient()
	if existing.GetSecret() == "" {
		return "", fmt.Errorf("dex client %s has no secret", id)
	}

	merged := mergeRedirectURIs(existing.GetRedirectUris(), redirectURIs)
	if existing.GetName() != name || len(merged) != len(existing.GetRedirectUris()) {
		updated, err := dex.UpdateClient(ctx, &api.UpdateClientReq{Id: id, Name: name, RedirectUris: merged})
		if err != nil {
			return "", fmt.Errorf("failed to update Dex client %s: %w", id, err)
		}
		if updated.GetNotFound() {
			return "", fmt.Errorf("dex client %s was deleted during update", id)
		}
	}

	return existing.GetSecret(), nil
}

// createClient returns nil if the client already exists
func createClient(ctx context.Context, dex api.DexClient, id, name string, redirectURIs []string) (*api.Client, error) {
	resp, err := dex.CreateClient(ctx, &api.CreateClientReq{Client: &api.Client{
		Id:           id,
		Name:         name,
		Secret:       rand.Text() + rand.Text(),
		RedirectUris: redirectURIs,
	}})
	if err != nil {
		return nil, fmt.Errorf("failed to create Dex client %s: %w", id, err)
	}
	if resp.GetAlreadyExists() {
		return nil, nil
	}
	return resp.GetClient(), nil
}

// mergeRedirectURIs returns existing with the missing desired redirect URIs appended
func mergeRedirectURIs(existing, desired []string) []string {
	merged := slices.Clone(existing)
	for _, uri := range desired {
		if !slices.Contains(merged, uri) {
			merged = append(merged, uri)
		}
	}
	return merged
}
