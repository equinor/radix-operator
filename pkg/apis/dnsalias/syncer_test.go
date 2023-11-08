package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type syncerTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	kubeUtil    *kube.Kube
	promClient  *prometheusfake.Clientset
	dnsConfig   *config.DNSConfig
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.dnsConfig = &config.DNSConfig{DNSZone: "test.radix.equinor.com"}
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
}

func (s *syncerTestSuite) createSyncer(radixDNSAlias *radixv1.RadixDNSAlias) dnsalias.Syncer {
	return dnsalias.NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.dnsConfig, radixDNSAlias)
}

type testIngress struct {
	appName   string
	envName   string
	domain    string
	host      string
	component string
	port      int32
}
type testDNSAlias struct {
	Domain      string
	Environment string
	Component   string
	Port        int32
}

type scenario struct {
	name            string
	expectedError   string
	dnsAlias        testDNSAlias
	dnsZone         string
	existingIngress map[string]testIngress
	expectedIngress map[string]testIngress
}

func (s *syncerTestSuite) Test_syncer_OnSync() {
	const (
		appName1   = "app1"
		appName2   = "app2"
		envName1   = "env1"
		envName2   = "env2"
		component1 = "component1"
		component2 = "component2"
		domain1    = "domain1"
		domain2    = "domain2"
		port8080   = 8080
		port9090   = 9090
		dnsZone1   = "test.radix.equinor.com"
	)

	scenarios := []scenario{
		{
			name:     "created an ingress",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "created additional ingress for another component",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component2.domain2.custom-domain": {appName: appName1, envName: envName1, domain: domain2, host: dnsalias.GetDNSAliasHost(domain2, dnsZone1), component: component2, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
				"component2.domain2.custom-domain": {appName: appName1, envName: envName1, domain: domain2, host: dnsalias.GetDNSAliasHost(domain2, dnsZone1), component: component2, port: port8080},
			},
		},
		{
			name:     "changed port changes port in existing ingress",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port9090},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port9090},
			},
		},
		{
			name:     "created additional ingress on another domain for the same component",
			dnsAlias: testDNSAlias{Domain: domain2, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain2, dnsZone1), component: component1, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
				"component1.domain2.custom-domain": {appName: appName1, envName: envName1, domain: domain2, host: dnsalias.GetDNSAliasHost(domain2, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "manually changed appName repaired",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName2, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component2, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "manually changed envName repaired",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName2, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component2, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "manually changed component repaired",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component2, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "manually changed port repaired",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port9090},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		{
			name:     "manually changed host repaired",
			dnsAlias: testDNSAlias{Domain: domain1, Environment: envName1, Component: component1, Port: port8080},
			dnsZone:  dnsZone1,
			existingIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: "/manually/edited/host", component: component1, port: port8080},
			},
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, domain: domain1, host: dnsalias.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			radixDNSAlias := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Domain, UID: uuid.NewUUID()},
				Spec: radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component, Port: ts.dnsAlias.Port}}
			s.Require().NoError(commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, ts.dnsAlias.Domain, radixDNSAlias.Spec), "create existing alias")
			cfg := &config.DNSConfig{DNSZone: ts.dnsZone}
			s.Require().NoError(registerExistingIngresses(s.kubeClient, ts.existingIngress, appName1, envName1, cfg), "create existing ingresses")
			syncer := s.createSyncer(radixDNSAlias)
			err := syncer.OnSync()
			commonTest.AssertError(s.T(), ts.expectedError, err)

			ingresses, err := s.kubeClient.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)

			// assert ingresses
			if ts.expectedIngress == nil {
				s.Require().Len(ingresses.Items, 0, "not expected ingresses")
				return
			}

			s.Require().Len(ingresses.Items, len(ts.expectedIngress), "not matching expected ingresses count")
			if len(ingresses.Items) == len(ts.expectedIngress) {
				for _, ingress := range ingresses.Items {
					if expectedIngress, ok := ts.expectedIngress[ingress.Name]; ok {
						s.Require().Len(ingress.Spec.Rules, 1, "rules count")
						s.Assert().Equal(expectedIngress.appName, ingress.GetLabels()[kube.RadixAppLabel], "app name")
						s.Assert().Equal(utils.GetEnvironmentNamespace(expectedIngress.appName, expectedIngress.envName), ingress.GetNamespace(), "namespace")
						s.Assert().Equal(expectedIngress.component, ingress.GetLabels()[kube.RadixComponentLabel], "component name")
						s.Assert().Equal(expectedIngress.host, ingress.Spec.Rules[0].Host, "rule host")
						s.Assert().Equal("/", ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path, "rule http path")
						s.Assert().Equal(expectedIngress.component, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name, "rule backend service name")
						s.Assert().Equal(expectedIngress.port, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "rule backend service port")
						if len(ingress.ObjectMeta.OwnerReferences) > 0 {
							ownerRef := ingress.ObjectMeta.OwnerReferences[0]
							s.Assert().Equal(radix.APIVersion, ownerRef.APIVersion, "ownerRef.APIVersion")
							s.Assert().Equal(radix.KindRadixDNSAlias, ownerRef.Kind, "ownerRef.Kind")
							s.Assert().Equal(radixDNSAlias.GetName(), ownerRef.Name, "ownerRef.Name")
							s.Assert().Equal(radixDNSAlias.GetUID(), ownerRef.UID, "ownerRef.UID")
							s.Assert().True(ownerRef.Controller != nil && *ownerRef.Controller, "ownerRef.Controller")
						}
						continue
					}
					assert.Fail(t, fmt.Sprintf("found not expected ingress %s for: appName %s, host %s, service %s, port %d",
						ingress.GetName(), ingress.GetLabels()[kube.RadixAppLabel], ingress.Spec.Rules[0].Host, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
						ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number))
				}
			}

		})
	}
}

func registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses map[string]testIngress, appNameForNamespace, envNameForNamespace string, config *config.DNSConfig) error {
	for name, ing := range testIngresses {
		ingress := dnsalias.BuildRadixDNSAliasIngress(ing.appName, ing.domain, ing.component, ing.port, nil, config)
		ingress.SetName(name) // override built name with expected name for test purpose
		_, err := dnsalias.CreateRadixDNSAliasIngress(kubeClient, appNameForNamespace, envNameForNamespace, ingress)
		if err != nil {
			return err
		}
	}
	return nil
}
