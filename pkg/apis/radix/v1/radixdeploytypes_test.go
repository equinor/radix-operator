package v1_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_RadixCommonDeployComponent_GetExternalDNS(t *testing.T) {
	var sut v1.RadixCommonDeployComponent

	// RadixDeployComponent tests
	sut = &v1.RadixDeployComponent{}
	assert.Empty(t, sut.GetExternalDNS())

	sut = &v1.RadixDeployComponent{DNSExternalAlias: []string{"foo.example.com", "bar.example.com"}}
	assert.ElementsMatch(
		t,
		[]v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseCertificateAutomation: false}, {FQDN: "bar.example.com", UseCertificateAutomation: false}},
		sut.GetExternalDNS(),
	)

	sut = &v1.RadixDeployComponent{ExternalDNS: []v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseCertificateAutomation: true}, {FQDN: "bar.example.com", UseCertificateAutomation: false}}}
	assert.ElementsMatch(
		t,
		[]v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseCertificateAutomation: true}, {FQDN: "bar.example.com", UseCertificateAutomation: false}},
		sut.GetExternalDNS(),
	)

	sut = &v1.RadixDeployComponent{
		DNSExternalAlias: []string{"foo.example.com", "bar.example.com"},
		ExternalDNS:      []v1.RadixDeployExternalDNS{{FQDN: "foo2.example.com", UseCertificateAutomation: true}, {FQDN: "bar2.example.com", UseCertificateAutomation: false}},
	}
	assert.ElementsMatch(
		t,
		[]v1.RadixDeployExternalDNS{{FQDN: "foo2.example.com", UseCertificateAutomation: true}, {FQDN: "bar2.example.com", UseCertificateAutomation: false}},
		sut.GetExternalDNS(),
	)

	// RadixDeployJobComponent tests
	sut = &v1.RadixDeployJobComponent{}
	assert.Empty(t, sut.GetExternalDNS())
}

func Test_RadixDeployComponent_GetPublicPortNumber(t *testing.T) {
	tests := map[string]struct {
		component    *v1.RadixDeployComponent
		expectedPort int32
		expectedOk   bool
	}{
		"nil component": {
			component:    nil,
			expectedPort: 0,
			expectedOk:   false,
		},
		"empty PublicPort": {
			component:    &v1.RadixDeployComponent{},
			expectedPort: 0,
			expectedOk:   false,
		},
		"PublicPort matches a port": {
			component: &v1.RadixDeployComponent{
				PublicPort: "http",
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8080}},
			},
			expectedPort: 8080,
			expectedOk:   true,
		},
		"PublicPort matches second port": {
			component: &v1.RadixDeployComponent{
				PublicPort: "http",
				Ports:      []v1.ComponentPort{{Name: "metrics", Port: 9090}, {Name: "http", Port: 8080}},
			},
			expectedPort: 8080,
			expectedOk:   true,
		},
		"PublicPort does not match any port": {
			component: &v1.RadixDeployComponent{
				PublicPort: "web",
				Ports:      []v1.ComponentPort{{Name: "http", Port: 8080}},
			},
			expectedPort: 0,
			expectedOk:   false,
		},
		"PublicPort set but no ports defined": {
			component: &v1.RadixDeployComponent{
				PublicPort: "http",
				Ports:      []v1.ComponentPort{},
			},
			expectedPort: 0,
			expectedOk:   false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			port, ok := tt.component.GetPublicPortNumber()
			assert.Equal(t, tt.expectedPort, port)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}

func Test_RadixDeployJobComponent_GetPublicPortNumber(t *testing.T) {
	port, ok := (&v1.RadixDeployJobComponent{}).GetPublicPortNumber()
	assert.Equal(t, int32(0), port)
	assert.False(t, ok)
}
