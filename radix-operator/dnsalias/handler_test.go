package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type handlerTestSuite struct {
	common.ControllerTestSuite
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}

func (s *handlerTestSuite) TearDownTest() {
	s.MockCtrl.Finish()
}

type testIngress struct {
	appName   string
	envName   string
	name      string
	host      string
	component string
	port      int32
}

func (s *handlerTestSuite) Test_IngressesForRadixDNSAliases() {
	const (
		appName              = "any-app1"
		envDev               = "dev"
		componentNameServer1 = "server1"
		server1Anyapp1DevDns = "server1-any-app1-dev.radix.equinor.com"
		port                 = 8080
	)
	var testScenarios = []struct {
		name                    string
		dnsAliases              []radixv1.DNSAlias
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
		existingIngress         []testIngress
		expectedIngress         map[string]testIngress
	}{
		{
			name: "no aliases, no existing RDA, no existing ingresses, no additional radix aliases, no additional ingresses",
		},
		{
			name:            "no aliases, no existing RDA, exist ingresses, no additional radix aliases, no additional ingresses",
			existingIngress: []testIngress{{appName: appName, envName: envDev, name: componentNameServer1, host: server1Anyapp1DevDns, component: componentNameServer1, port: port}},
			expectedIngress: map[string]testIngress{componentNameServer1: {appName: appName, envName: envDev, name: componentNameServer1, host: server1Anyapp1DevDns, component: componentNameServer1, port: port}},
		},
		{
			name: "multiple aliases, no existing RDA, no existing ingresses, additional radix aliases, additional ingresses",
			dnsAliases: []radixv1.DNSAlias{
				{Domain: "domain1", Environment: envDev, Component: componentNameServer1},
				{Domain: "domain2", Environment: envDev, Component: componentNameServer1},
			},
			expectedIngress: map[string]testIngress{
				"server1.domain1.custom-domain": {appName: appName, envName: envDev, name: "server1.domain1.custom-domain", host: "domain1.custom-domain.radix.equinor.com", component: componentNameServer1, port: port},
				"server1.domain2.custom-domain": {appName: appName, envName: envDev, name: "server1.domain2.custom-domain", host: "domain2.custom-domain.radix.equinor.com", component: componentNameServer1, port: port},
			},
		},
	}
	s.SetupTest()

	for _, ts := range testScenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			ra := utils.ARadixApplication().WithAppName(appName).WithDNSAlias(ts.dnsAliases...).BuildRA()
			_, err := s.RadixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), ra, metav1.CreateOptions{})
			require.NoError(t, err)
			require.NoError(t, registerExistingIngresses(s.KubeClient, ts.existingIngress), "create existing ingresses")

			ingresses, err := s.KubeClient.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			// assert ingresses
			if ts.expectedIngress == nil {
				assert.Len(t, ingresses.Items, 0, "not expected ingresses")
			} else {
				assert.Len(t, ingresses.Items, len(ts.expectedIngress), "not matching expected ingresses count")
				if len(ingresses.Items) == len(ts.expectedIngress) {
					for _, ingress := range ingresses.Items {
						if expectedIngress, ok := ts.expectedIngress[ingress.Name]; ok {
							require.Len(t, ingress.Spec.Rules, 1, "rules count")
							assert.Equal(t, expectedIngress.appName, ingress.GetLabels()[kube.RadixAppLabel], "app name")
							assert.Equal(t, utils.GetEnvironmentNamespace(expectedIngress.appName, expectedIngress.envName), ingress.GetNamespace(), "namespace")
							assert.Equal(t, expectedIngress.component, ingress.GetLabels()[kube.RadixComponentLabel], "component name")
							assert.Equal(t, expectedIngress.host, ingress.Spec.Rules[0].Host, "rule host")
							assert.Equal(t, "/", ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path, "rule http path")
							assert.Equal(t, expectedIngress.component, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name, "rule backend service name")
							assert.Equal(t, expectedIngress.port, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "rule backend service port")
						} else {
							assert.Failf(t, "found not expected ingress %s: appName %s, host %s, service %s, port %d", ingress.GetName(), ingress.GetLabels()[kube.RadixAppLabel], ingress.Spec.Rules[0].Host, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name, &ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port)
						}
					}
				}
			}
		})
	}
}

func registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses []testIngress) error {
	for _, ing := range testIngresses {
		_, err := internal.CreateRadixDNSAliasIngress(kubeClient, ing.appName, ing.envName, internal.BuildRadixDNSAliasIngress(ing.appName, ing.name, ing.component, ing.port))
		if err != nil {
			return err
		}
	}
	return nil
}
