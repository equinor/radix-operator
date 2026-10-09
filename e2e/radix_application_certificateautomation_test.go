package e2e

import (
	"maps"
	"slices"
	"strings"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// warningRecorder collects warnings returned by the API server (e.g. from admission webhooks).
type warningRecorder []string

func (w *warningRecorder) HandleWarningHeader(_ int, _ string, text string) {
	*w = append(*w, text)
}

// getClientWithWarningRecorder returns a client that records API server warnings.
func getClientWithWarningRecorder(t *testing.T) (client.Client, *warningRecorder) {
	var warnings warningRecorder
	cfg := rest.CopyConfig(testManager.GetConfig())
	cfg.WarningHandler = &warnings
	c, err := client.New(cfg, client.Options{Scheme: testManager.GetScheme()})
	require.NoError(t, err)
	return c, &warnings
}

// TestRadixApplicationCertificateAutomationValidation tests the certificate automation validation of external aliases.
// Issuer cases are derived from e2eCertificateIssuers and e2eDefaultCertificateIssuer.
func TestRadixApplicationCertificateAutomationValidation(t *testing.T) {
	c, warnings := getClientWithWarningRecorder(t)
	appName := "test-cert-automation"
	appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)

	const (
		envName       = "dev"
		componentName = "web"
		// Accepted by the CRD, but not configured as an issuer in the e2e cluster
		notConfiguredIssuer = "non-existing-issuer"
	)

	externalAlias := func(alias string, useAutomation bool, certAutomation *v1.CertificateAutomation) v1.ExternalAlias {
		return v1.ExternalAlias{
			Alias:                    alias,
			Environment:              envName,
			Component:                componentName,
			UseCertificateAutomation: useAutomation,
			CertificateAutomation:    certAutomation,
		}
	}

	configuredIssuers := slices.Sorted(maps.Keys(e2eCertificateIssuers))
	mixedAliases := []v1.ExternalAlias{
		externalAlias("mixed-default.example.com", true, nil),
		externalAlias("mixed-disabled.example.com", false, nil),
	}
	for _, issuer := range configuredIssuers {
		mixedAliases = append(mixedAliases, externalAlias("mixed-"+issuer+".example.com", true, &v1.CertificateAutomation{Issuer: issuer}))
	}

	type testCase struct {
		name                    string
		externalAliases         []v1.ExternalAlias
		shouldError             bool
		expectedErrContains     string
		expectedWarningContains string
	}
	testCases := []testCase{
		{
			name:            "valid - no external aliases",
			externalAliases: nil,
		},
		{
			name:            "valid - automation disabled without issuer",
			externalAliases: []v1.ExternalAlias{externalAlias("disabled.example.com", false, nil)},
		},
		{
			name:                    "valid with warning - automation disabled with a configured issuer",
			externalAliases:         []v1.ExternalAlias{externalAlias("disabled-issuer.example.com", false, &v1.CertificateAutomation{Issuer: e2eDefaultCertificateIssuer})},
			expectedWarningContains: "external alias disabled-issuer.example.com: unused certificate automation configuration",
		},
		{
			name:            "valid - automation enabled without issuer falls back to default issuer",
			externalAliases: []v1.ExternalAlias{externalAlias("default.example.com", true, nil)},
		},
		{
			name:            "valid - multiple aliases mixing default issuer, every configured issuer and disabled automation",
			externalAliases: mixedAliases,
		},
		{
			name:                "invalid - certificate automation with empty issuer",
			externalAliases:     []v1.ExternalAlias{externalAlias("notconfigured.example.com", true, &v1.CertificateAutomation{Issuer: ""})},
			shouldError:         true,
			expectedErrContains: "some validation rules were not checked because the object was invalid; correct the existing errors to complete validation",
		},
		{
			name:                "invalid - certificate automation without issuer",
			externalAliases:     []v1.ExternalAlias{externalAlias("notconfigured.example.com", true, &v1.CertificateAutomation{})},
			shouldError:         true,
			expectedErrContains: "some validation rules were not checked because the object was invalid; correct the existing errors to complete validation",
		},
		{
			name:                "invalid - automation enabled with issuer not configured in cluster (webhook)",
			externalAliases:     []v1.ExternalAlias{externalAlias("notconfigured.example.com", true, &v1.CertificateAutomation{Issuer: notConfiguredIssuer})},
			shouldError:         true,
			expectedErrContains: "external alias notconfigured.example.com: invalid selected certificate automation issuer",
		},
		{
			name: "invalid - one of multiple aliases uses issuer not configured in cluster (webhook)",
			externalAliases: []v1.ExternalAlias{
				externalAlias("ok.example.com", true, nil),
				externalAlias("bad.example.com", true, &v1.CertificateAutomation{Issuer: notConfiguredIssuer}),
			},
			shouldError:         true,
			expectedErrContains: "external alias bad.example.com: invalid selected certificate automation issuer",
		},
	}
	for _, issuer := range configuredIssuers {
		testCases = append(testCases, testCase{
			name:            "valid - automation enabled with explicit configured issuer " + issuer,
			externalAliases: []v1.ExternalAlias{externalAlias("explicit-"+issuer+".example.com", true, &v1.CertificateAutomation{Issuer: issuer})},
		})
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ra := &v1.RadixApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: appNamespace,
				},
				Spec: v1.RadixApplicationSpec{
					Environments: []v1.Environment{{Name: envName}},
					Components: []v1.RadixComponent{
						{
							Name:       componentName,
							Ports:      []v1.ComponentPort{{Name: "http", Port: 8080}},
							PublicPort: "http",
						},
					},
					DNSExternalAlias: tc.externalAliases,
				},
			}

			*warnings = nil
			err := c.Create(t.Context(), ra, client.DryRunAll)

			if tc.expectedWarningContains != "" {
				assert.True(t, slices.ContainsFunc(*warnings, func(w string) bool {
					return strings.Contains(w, tc.expectedWarningContains)
				}), "expected warning %q, got %v", tc.expectedWarningContains, *warnings)
			}

			if !tc.shouldError {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.expectedErrContains != "" {
				assert.ErrorContains(t, err, tc.expectedErrContains)
			}
		})
	}
}
