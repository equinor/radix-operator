package dex

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	api "github.com/dexidp/dex/api/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/equinor/radix-operator/easyauth/internal/dex/dextest"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
)

const (
	clientID         = "easyauth.01JABCDEFGHJKMNPQRSTVWXYZ0"
	rotationInterval = 90 * 24 * time.Hour
)

// secretCreatedAgo returns a secret created d ago.
func secretCreatedAgo(d time.Duration) string {
	return NewSecret(time.Now().Add(-d))
}

func desiredClient(name string, redirectURIs ...string) Client {
	return Client{ID: clientID, Name: name, RedirectURIs: redirectURIs}
}

func ensure(dexClient api.DexClient, name string, redirectURIs ...string) (string, time.Time, error) {
	return NewClientManager(dexClient).EnsureClient(context.Background(), desiredClient(name, redirectURIs...), rotationInterval)
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, IsNotFound(status.Error(codes.Unknown, "not found")))
	assert.True(t, IsNotFound(status.Error(codes.NotFound, "client missing")))
	assert.False(t, IsNotFound(status.Error(codes.Unknown, "database unavailable")))
	assert.False(t, IsNotFound(status.Error(codes.Unavailable, "not found")))
	assert.False(t, IsNotFound(errors.New("not found")))
	assert.False(t, IsNotFound(nil))
}

func TestEnsureClient_CreatesMissingClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fakeDex := dextest.NewFakeDex()
		uri := "https://a.example.com/oauth2/callback"

		secret, rotateAt, err := ensure(fakeDex, "my-app", uri)
		require.NoError(t, err)

		created := fakeDex.Client(clientID)
		require.NotNil(t, created)
		assert.NotEmpty(t, secret)
		assert.Equal(t, secret, created.Secret)
		assert.WithinDuration(t, time.Now().Add(rotationInterval), rotateAt, 0)
		assert.Equal(t, "my-app", created.Name)
		assert.Equal(t, []string{uri}, created.RedirectUris)
		assert.False(t, created.Public)
	})
}

func TestEnsureClient_KeepsSecretAndMergesRedirectURIs(t *testing.T) {
	existingSecret := secretCreatedAgo(24 * time.Hour)
	fakeDex := dextest.NewFakeDex(&api.Client{
		Id:           clientID,
		Name:         "old-name",
		Secret:       existingSecret,
		RedirectUris: []string{"https://other-cluster.example.com/oauth2/callback", "https://a.example.com/oauth2/callback"},
	})

	secret, _, err := ensure(fakeDex, "my-app",
		"https://a.example.com/oauth2/callback",
		"https://b.example.com/oauth2/callback",
	)
	require.NoError(t, err)

	assert.Equal(t, existingSecret, secret)
	updated := fakeDex.Client(clientID)
	assert.Equal(t, existingSecret, updated.Secret)
	assert.Equal(t, "my-app", updated.Name)
	assert.Equal(t, []string{
		"https://other-cluster.example.com/oauth2/callback",
		"https://a.example.com/oauth2/callback",
		"https://b.example.com/oauth2/callback",
	}, updated.RedirectUris, "existing redirect URIs must be kept")
}

func TestEnsureClient_DoesNotUpdateUnchangedClient(t *testing.T) {
	existingSecret := secretCreatedAgo(24 * time.Hour)
	fakeDex := dextest.NewFakeDex(&api.Client{
		Id:           clientID,
		Name:         "my-app",
		Secret:       existingSecret,
		RedirectUris: []string{"https://a.example.com/oauth2/callback", "https://b.example.com/oauth2/callback"},
	})

	secret, _, err := ensure(fakeDex, "my-app", "https://b.example.com/oauth2/callback")
	require.NoError(t, err)

	assert.Equal(t, existingSecret, secret)
	assert.Zero(t, fakeDex.Calls("UpdateClient"))
	assert.Zero(t, fakeDex.Calls("CreateClient"))
}

func TestEnsureClient_RotatesDueSecret(t *testing.T) {
	tests := map[string]func() string{
		"older than the rotation interval": func() string { return secretCreatedAgo(rotationInterval) },
		"without creation time":            func() string { return "secret-from-an-older-version" },
	}
	for name, newOldSecret := range tests {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				oldSecret := newOldSecret()
				fakeDex := dextest.NewFakeDex(&api.Client{
					Id:           clientID,
					Name:         "my-app",
					Secret:       oldSecret,
					RedirectUris: []string{"https://other-cluster.example.com/oauth2/callback"},
				})

				secret, rotateAt, err := ensure(fakeDex, "my-app", "https://a.example.com/oauth2/callback")
				require.NoError(t, err)

				assert.NotEqual(t, oldSecret, secret)
				assert.WithinDuration(t, time.Now().Add(rotationInterval), rotateAt, 0)
				rotated := fakeDex.Client(clientID)
				assert.Equal(t, secret, rotated.Secret)
				assert.Equal(t, "my-app", rotated.Name)
				assert.Equal(t, []string{
					"https://other-cluster.example.com/oauth2/callback",
					"https://a.example.com/oauth2/callback",
				}, rotated.RedirectUris, "redirect URIs of other clusters must be kept")
				assert.Equal(t, 1, fakeDex.Calls("DeleteClient"))
			})
		})
	}
}

func TestEnsureClient_DoesNotRotateSecretBeforeInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notDue := secretCreatedAgo(rotationInterval - time.Minute)
		fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: "my-app", Secret: notDue})

		secret, rotateAt, err := ensure(fakeDex, "my-app")
		require.NoError(t, err)

		assert.Equal(t, notDue, secret)
		assert.WithinDuration(t, time.Now().Add(time.Minute), rotateAt, 0)
		assert.Zero(t, fakeDex.Calls("DeleteClient"))
	})
}

// createdConcurrently returns not found on the first GetClient, as if another cluster created the client right after
type createdConcurrently struct {
	*dextest.FakeDex
	gets int
}

func (c *createdConcurrently) GetClient(ctx context.Context, in *api.GetClientReq, opts ...grpc.CallOption) (*api.GetClientResp, error) {
	c.gets++
	if c.gets == 1 {
		return nil, status.Error(codes.Unknown, "not found")
	}
	return c.FakeDex.GetClient(ctx, in, opts...)
}

func TestEnsureClient_UsesClientCreatedConcurrently(t *testing.T) {
	existingSecret := secretCreatedAgo(24 * time.Hour)
	fakeDex := &createdConcurrently{FakeDex: dextest.NewFakeDex(&api.Client{Id: clientID, Name: "my-app", Secret: existingSecret})}

	secret, _, err := ensure(fakeDex, "my-app")
	require.NoError(t, err)
	assert.Equal(t, existingSecret, secret)
}

func TestEnsureClient_ReturnsDexErrors(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	fakeDex.SetError(status.Error(codes.Unavailable, "connection refused"))

	_, _, err := ensure(fakeDex, "my-app")
	require.Error(t, err)
	assert.Zero(t, fakeDex.Calls("CreateClient"), "must not create a client when Dex returns other errors than not found")
}

func TestEnsureClient_FailsOnClientWithoutSecret(t *testing.T) {
	fakeDex := dextest.NewFakeDex(&api.Client{Id: clientID, Name: "my-app", Public: true})

	_, _, err := ensure(fakeDex, "my-app")
	assert.Error(t, err)
}

// rotatedConcurrently creates a new client right after it is deleted, as if another cluster rotated it at the same time
type rotatedConcurrently struct {
	*dextest.FakeDex
	otherSecret string
}

func (c *rotatedConcurrently) DeleteClient(ctx context.Context, in *api.DeleteClientReq, opts ...grpc.CallOption) (*api.DeleteClientResp, error) {
	resp, err := c.FakeDex.DeleteClient(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	_, err = c.FakeDex.CreateClient(ctx, &api.CreateClientReq{Client: &api.Client{Id: in.Id, Name: "my-app", Secret: c.otherSecret}})
	return resp, err
}

func TestEnsureClient_UsesSecretRotatedConcurrently(t *testing.T) {
	otherSecret := secretCreatedAgo(0)
	fakeDex := &rotatedConcurrently{
		FakeDex:     dextest.NewFakeDex(&api.Client{Id: clientID, Name: "my-app", Secret: secretCreatedAgo(rotationInterval)}),
		otherSecret: otherSecret,
	}

	secret, _, err := ensure(fakeDex, "my-app")
	require.NoError(t, err)
	assert.Equal(t, otherSecret, secret)
}

func TestSecretCreatedAt(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	createdAt, ok := SecretCreatedAt(NewSecret(now))
	require.True(t, ok)
	assert.Equal(t, now, createdAt)

	for _, secret := range []string{"", "no-separator", "not-a-ulid.random", "8ZZZZZZZZZZZZZZZZZZZZZZZZZ.random"} {
		_, ok := SecretCreatedAt(secret)
		assert.False(t, ok, secret)
	}
}

func TestNewSecret_IsUnique(t *testing.T) {
	now := time.Now()
	assert.NotEqual(t, NewSecret(now), NewSecret(now))
	assert.GreaterOrEqual(t, len(NewSecret(now)), 50)
}

func TestClientIDForApp(t *testing.T) {
	id := ulid.Make()
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(
		&radixv1.RadixRegistration{Name: "with-id", Spec: radixv1.RadixRegistrationSpec{AppID: radixv1.ULID{ULID: id}}},
		&radixv1.RadixRegistration{Name: "without-id"},
	).Build()

	got, err := ClientIDForApp(context.Background(), c, "with-id")
	require.NoError(t, err)
	assert.Equal(t, "easyauth."+id.String(), got)

	_, err = ClientIDForApp(context.Background(), c, "without-id")
	assert.Error(t, err)

	_, err = ClientIDForApp(context.Background(), c, "missing")
	assert.Error(t, err)
}

// versionServer is a Dex gRPC server that only implements GetVersion
type versionServer struct {
	api.UnimplementedDexServer
}

func (versionServer) GetVersion(context.Context, *api.VersionReq) (*api.VersionResp, error) {
	return &api.VersionResp{Server: "test"}, nil
}

func TestNewClient_AuthenticatesWithClientCertificate(t *testing.T) {
	ca := newCA(t)
	certsDir := t.TempDir()
	writeCerts(t, certsDir, ca, ca.issue(t, x509.ExtKeyUsageClientAuth))
	address := startServer(t, ca, ca.issue(t, x509.ExtKeyUsageServerAuth, "localhost"))

	dexClient, conn, err := NewClient(address, certsDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	resp, err := dexClient.GetVersion(context.Background(), &api.VersionReq{})
	require.NoError(t, err)
	assert.Equal(t, "test", resp.Server)
}

func TestNewClient_RejectedWithCertificateFromOtherCA(t *testing.T) {
	serverCA, otherCA := newCA(t), newCA(t)
	certsDir := t.TempDir()
	// Trusts the server, but the client certificate is not signed by the CA the server trusts
	writeCerts(t, certsDir, serverCA, otherCA.issue(t, x509.ExtKeyUsageClientAuth))
	address := startServer(t, serverCA, serverCA.issue(t, x509.ExtKeyUsageServerAuth, "localhost"))

	dexClient, conn, err := NewClient(address, certsDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = dexClient.GetVersion(ctx, &api.VersionReq{})
	assert.Error(t, err)
}

func TestNewClient_FailsWithoutCertificates(t *testing.T) {
	_, _, err := NewClient("localhost:5557", t.TempDir())
	assert.Error(t, err)
}

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCA{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func (ca *testCA) issue(t *testing.T, usage x509.ExtKeyUsage, dnsNames ...string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test"},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	require.NoError(t, err)
	return cert
}

// writeCerts writes the files cert-manager writes to the client certificate Secret
func writeCerts(t *testing.T, dir string, ca *testCA, cert tls.Certificate) {
	t.Helper()
	keyDER, err := x509.MarshalECPrivateKey(cert.PrivateKey.(*ecdsa.PrivateKey))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ca.crt"), ca.pem, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
}

// startServer starts a gRPC server that requires a client certificate signed by ca, like Dex with tlsClientCA
func startServer(t *testing.T, ca *testCA, cert tls.Certificate) string {
	t.Helper()
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca.cert)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	})))
	api.RegisterDexServer(server, versionServer{})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return net.JoinHostPort("localhost", port)
}
