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
	"strings"
	"time"

	api "github.com/dexidp/dex/api/v2"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	oauthutil "github.com/equinor/radix-operator/pkg/apis/utils/oauth"
)

// ClientIDPrefix identifies Dex clients owned by EasyAuth, since Dex clients have no labels or metadata
const ClientIDPrefix = "easyauth."

// recreateTimeout is how long rotateSecret waits to create the client again after deleting it
const recreateTimeout = 10 * time.Second

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

// RedirectURI returns the OAuth2 proxy callback URL for a public host. The path is built from proxyPrefix the same way
// as radix-operator builds OAUTH2_PROXY_REDIRECT_URL.
func RedirectURI(host, proxyPrefix string) string {
	return fmt.Sprintf("https://%s%s/callback", host, oauthutil.SanitizePathPrefix(proxyPrefix))
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

// Client is the desired state of the Dex client of an application
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
}

// ClientManager manages the clients of applications in Dex
type ClientManager struct {
	dex api.DexClient
}

// NewClientManager returns a ClientManager that uses the Dex gRPC API
func NewClientManager(dex api.DexClient) *ClientManager {
	return &ClientManager{dex: dex}
}

// EnsureClient creates the client if it doesn't exist, and returns the client secret stored in Dex and when it must be
// rotated. An existing client keeps its secret, while its name and redirect URIs are updated to match desired.
// A secret older than secretRotationInterval, or without a creation time, is rotated.
func (m *ClientManager) EnsureClient(ctx context.Context, desired Client, secretRotationInterval time.Duration) (secret string, rotateAt time.Time, err error) {
	existing, err := m.getOrCreateClient(ctx, desired)
	if err != nil {
		return "", time.Time{}, err
	}

	if time.Now().After(rotationTime(existing.GetSecret(), secretRotationInterval)) {
		secret, err := m.rotateSecret(ctx, desired)
		if err != nil {
			return "", time.Time{}, err
		}
		return secret, rotationTime(secret, secretRotationInterval), nil
	}

	if existing.GetName() != desired.Name || !slices.Equal(desired.RedirectURIs, existing.GetRedirectUris()) {
		updated, err := m.dex.UpdateClient(ctx, &api.UpdateClientReq{Id: desired.ID, Name: desired.Name, RedirectUris: desired.RedirectURIs})
		if err != nil {
			return "", time.Time{}, fmt.Errorf("failed to update Dex client %s: %w", desired.ID, err)
		}
		if updated.GetNotFound() {
			return "", time.Time{}, fmt.Errorf("dex client %s was deleted during update", desired.ID)
		}
	}

	return existing.GetSecret(), rotationTime(existing.GetSecret(), secretRotationInterval), nil
}

// rotateSecret replaces the secret of the client. Dex can't change the secret of an existing client, so the client is
// deleted and created again with the same ID, name and redirect URIs. Once deleted, the client is created on a context
// that is not cancelled with ctx, so a cancellation never leaves the client deleted. If creating still fails, the next
// EnsureClient creates the client again from the current desired state.
func (m *ClientManager) rotateSecret(ctx context.Context, desired Client) (string, error) {
	if _, err := m.dex.DeleteClient(ctx, &api.DeleteClientReq{Id: desired.ID}); err != nil {
		return "", fmt.Errorf("failed to delete Dex client %s for secret rotation: %w", desired.ID, err)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recreateTimeout)
	defer cancel()
	created, err := createClient(ctx, m.dex, desired.ID, desired.Name, desired.RedirectURIs)
	if err != nil {
		return "", err
	}
	log.Ctx(ctx).Info().Str("clientId", desired.ID).Msg("Rotated Dex client secret")
	return created.GetSecret(), nil
}

// getOrCreateClient returns the client, creating it if it doesn't exist
func (m *ClientManager) getOrCreateClient(ctx context.Context, desired Client) (*api.Client, error) {
	existing, err := m.getClient(ctx, desired.ID)
	if err != nil {
		if !IsNotFound(err) {
			return nil, err
		}

		existing, err = createClient(ctx, m.dex, desired.ID, desired.Name, desired.RedirectURIs)
		if err != nil {
			return nil, err
		}
	}

	return existing, nil
}

func (m *ClientManager) getClient(ctx context.Context, id string) (*api.Client, error) {
	resp, err := m.dex.GetClient(ctx, &api.GetClientReq{Id: id})
	if err != nil {
		if IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get Dex client %s: %w", id, err)
	}
	if resp == nil || resp.GetClient() == nil {
		return nil, fmt.Errorf("dex returned no client for %s", id)
	}
	return resp.GetClient(), nil
}

func createClient(ctx context.Context, dex api.DexClient, id, name string, redirectURIs []string) (*api.Client, error) {
	resp, err := dex.CreateClient(ctx, &api.CreateClientReq{Client: &api.Client{
		Id:           id,
		Name:         name,
		Secret:       NewSecret(time.Now()),
		RedirectUris: redirectURIs,
	}})
	if err != nil {
		return nil, fmt.Errorf("failed to create Dex client %s: %w", id, err)
	}
	if resp.GetAlreadyExists() {
		return nil, fmt.Errorf("failed to create Dex client %s: already exists", id)
	}
	if resp == nil || resp.GetClient() == nil {
		return nil, fmt.Errorf("dex returned no client after creating %s", id)
	}
	created := resp.GetClient()
	if created.GetSecret() == "" {
		return nil, fmt.Errorf("dex client %s has no secret after creation", id)
	}
	return created, nil
}

// NewSecret returns a secret that starts with a ULID with createdAt as the creation time, since Dex clients have no metadata
// to store it in. This lets every cluster sharing the Dex database tell when the secret must be rotated.
func NewSecret(createdAt time.Time) string {
	return ulid.MustNew(ulid.Timestamp(createdAt), rand.Reader).String() + "." + rand.Text()
}

// SecretCreatedAt returns when a secret created by NewSecret was created
func SecretCreatedAt(secret string) (time.Time, bool) {
	prefix, _, ok := strings.Cut(secret, ".")
	if !ok {
		return time.Time{}, false
	}
	id, err := ulid.ParseStrict(prefix)
	if err != nil {
		return time.Time{}, false
	}
	return ulid.Time(id.Time()).UTC(), true
}

// rotationTime returns when a secret must be rotated. Secrets without a creation time must be rotated now.
func rotationTime(secret string, secretRotationInterval time.Duration) time.Time {
	createdAt, ok := SecretCreatedAt(secret)
	if !ok {
		return time.Time{}
	}
	return createdAt.Add(secretRotationInterval)
}
