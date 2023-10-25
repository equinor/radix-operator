package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
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
		componentPort1       = 8080
		dnsZone1             = "test.radix.equinor.com"
	)
	var testScenarios = []struct {
		name                    string
		dnsAlias                radixv1.DNSAlias
		dnsZone                 string
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
		existingIngress         []testIngress
		expectedIngress         map[string]testIngress
	}{
		{
			name:     "new alias, no existing RDA, no existing ingresses, additional radix aliases, additional ingresses",
			dnsAlias: radixv1.DNSAlias{Domain: "domain1", Environment: envDev, Component: componentNameServer1},
			dnsZone:  dnsZone1,
			expectedIngress: map[string]testIngress{
				"server1.domain1.custom-domain": {appName: appName, envName: envDev, name: "server1.domain1.custom-domain", host: internal.GetDNSAliasHost("domain1", dnsZone1), component: componentNameServer1, port: componentPort1},
				"server1.domain2.custom-domain": {appName: appName, envName: envDev, name: "server1.domain2.custom-domain", host: internal.GetDNSAliasHost("domain2", dnsZone1), component: componentNameServer1, port: componentPort1},
			},
		},
	}

	for _, ts := range testScenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			config := &config.ClusterConfig{DNSZone: ts.dnsZone}
			ra := utils.ARadixApplication().WithAppName(appName).
				WithEnvironment(ts.dnsAlias.Environment, "branch1").
				WithComponent(utils.NewApplicationComponentBuilder().WithName(ts.dnsAlias.Component).WithPort("http", componentPort1)).BuildRA()
			_, err := s.RadixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), ra, metav1.CreateOptions{})
			require.NoError(t, err)
			require.NoError(t, registerExistingIngresses(s.KubeClient, ts.existingIngress, config), "create existing ingresses")
			require.NoError(t, registerExistingRadixDNSAliases(s.RadixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")

			require.NoError(t, registerExistingRadixDNSAliases(s.RadixClient,
				map[string]radixv1.RadixDNSAliasSpec{ts.dnsAlias.Domain: internal.BuildRadixDNSAlias(appName, ts.dnsAlias.Component, ts.dnsAlias.Environment, ts.dnsAlias.Domain).Spec}),
				"create new or updated RadixDNSAlias")
			handlerSynced := false
			handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, nil, func(synced bool) { handlerSynced = synced })
			err = handler.Sync("", ts.dnsAlias.Domain, s.EventRecorder)
			require.NoError(s.T(), err)
			require.True(s.T(), handlerSynced, "Handler should be synced")

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

func registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses []testIngress, config *config.ClusterConfig) error {
	for _, ing := range testIngresses {
		_, err := internal.CreateRadixDNSAliasIngress(kubeClient, ing.appName, ing.envName, internal.BuildRadixDNSAliasIngress(ing.appName, ing.name, ing.component, ing.port, config))
		if err != nil {
			return err
		}
	}
	return nil
}

func registerExistingRadixDNSAliases(radixClient radixclient.Interface, radixDNSAliasesMap map[string]radixv1.RadixDNSAliasSpec) error {
	for domain, rdaSpec := range radixDNSAliasesMap {
		_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.TODO(),
			&radixv1.RadixDNSAlias{
				ObjectMeta: metav1.ObjectMeta{
					Name:   domain,
					Labels: labels.Merge(labels.ForApplicationName(rdaSpec.AppName), labels.ForComponentName(rdaSpec.Component)),
				},
				Spec: rdaSpec,
			}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
