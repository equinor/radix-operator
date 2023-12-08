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
		[]v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseAutomation: false}, {FQDN: "bar.example.com", UseAutomation: false}},
		sut.GetExternalDNS(),
	)

	sut = &v1.RadixDeployComponent{ExternalDNS: []v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseAutomation: true}, {FQDN: "bar.example.com", UseAutomation: false}}}
	assert.ElementsMatch(
		t,
		[]v1.RadixDeployExternalDNS{{FQDN: "foo.example.com", UseAutomation: true}, {FQDN: "bar.example.com", UseAutomation: false}},
		sut.GetExternalDNS(),
	)

	sut = &v1.RadixDeployComponent{
		DNSExternalAlias: []string{"foo.example.com", "bar.example.com"},
		ExternalDNS:      []v1.RadixDeployExternalDNS{{FQDN: "foo2.example.com", UseAutomation: true}, {FQDN: "bar2.example.com", UseAutomation: false}},
	}
	assert.ElementsMatch(
		t,
		[]v1.RadixDeployExternalDNS{{FQDN: "foo2.example.com", UseAutomation: true}, {FQDN: "bar2.example.com", UseAutomation: false}},
		sut.GetExternalDNS(),
	)

	// RadixDeployJobComponent tests
	sut = &v1.RadixDeployJobComponent{}
	assert.Empty(t, sut.GetExternalDNS())
}
