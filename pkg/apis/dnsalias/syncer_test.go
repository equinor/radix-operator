package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/radix-operator/config"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type syncerTestSuite struct {
	suite.Suite
	kubeClient    *kubefake.Clientset
	radixClient   *radixfake.Clientset
	kubeUtil      *kube.Kube
	promClient    *prometheusfake.Clientset
	clusterConfig *config.ClusterConfig
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.clusterConfig = &config.ClusterConfig{DNSZone: "test.radix.equinor.com"}
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
}

func (s *syncerTestSuite) createSyncer(radixDNSAlias *radixv1.RadixDNSAlias) dnsalias.Syncer {
	return dnsalias.NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.clusterConfig, radixDNSAlias)
}

type testIngress struct {
	appName   string
	envName   string
	name      string
	host      string
	component string
	port      int32
}

type modifyComponentBuilderFunc func(builder utils.RadixApplicationComponentBuilder) (utils.RadixApplicationComponentBuilder, bool)

type scenario struct {
	name                        string
	expectedError               string
	missingRadixApplication     bool
	dnsAlias                    radixv1.DNSAlias
	dnsZone                     string
	existingRadixDNSAliases     map[string]radixv1.RadixDNSAliasSpec
	existingIngress             []testIngress
	expectedIngress             map[string]testIngress
	applicationComponentBuilder utils.RadixApplicationComponentBuilder
}

func (s *syncerTestSuite) Test_syncer_OnSync() {
	const (
		appName1   = "app1"
		envName1   = "env1"
		component1 = "component1"
		domain1    = "domain1"
		domain2    = "domain2"
		portHttp   = "http"
		port8080   = 8080
		dnsZone1   = "test.radix.equinor.com"
	)

	scenarios := []scenario{
		{
			name:                        "no radix application",
			missingRadixApplication:     true,
			applicationComponentBuilder: getRandomComponentBuilder(),
			expectedError:               "not found the Radix application app1 for the DNS alias domain1",
			dnsAlias:                    radixv1.DNSAlias{Domain: "domain1", Environment: envName1, Component: component1},
		},
		{
			name:                        "created an ingress",
			dnsAlias:                    radixv1.DNSAlias{Domain: domain1, Environment: envName1, Component: component1},
			dnsZone:                     dnsZone1,
			applicationComponentBuilder: utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portHttp, port8080).WithPublicPort(portHttp),
			expectedIngress: map[string]testIngress{
				"component1.domain1.custom-domain": {appName: appName1, envName: envName1, name: "component1.domain1.custom-domain", host: internal.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
			},
		},
		// {
		// 	name:                    "created additional ingress",
		// 	dnsAlias:                radixv1.DNSAlias{Domain: domain1, Environment: envName1, Component: component1},
		// 	dnsZone:                 dnsZone1,
		// 	expectedIngress: map[string]testIngress{
		// 		"component1.domain1.custom-domain": {appName: appName1, envName: envName1, name: "component1.domain1.custom-domain", host: internal.GetDNSAliasHost(domain1, dnsZone1), component: component1, port: port8080},
		// 		"component1.domain2.custom-domain": {appName: appName1, envName: envName1, name: "component1.domain2.custom-domain", host: internal.GetDNSAliasHost(domain2, dnsZone1), component: component1, port: port8080},
		// 	},
		// },
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			if !ts.missingRadixApplication {
				ra := utils.NewRadixApplicationBuilder().WithAppName(appName1).WithEnvironment(envName1, "master").
					WithComponents(ts.applicationComponentBuilder, getRandomComponentBuilder()).BuildRA()
				_, err := s.radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName1)).Create(context.Background(), ra, metav1.CreateOptions{})
				s.NoError(err)
			}
			config := &config.ClusterConfig{DNSZone: ts.dnsZone}
			radixDNSAlias := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Domain},
				Spec: radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component}}
			require.NoError(t, registerExistingRadixDNSAlias(s.radixClient, radixDNSAlias), "create existing alias")
			require.NoError(t, registerExistingIngresses(s.kubeClient, ts.existingIngress, config), "create existing ingresses")
			syncer := s.createSyncer(radixDNSAlias)
			err := syncer.OnSync()
			test.AssertError(s.T(), ts.expectedError, err)

			ingresses, err := s.kubeClient.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			// assert ingresses
			if ts.expectedIngress == nil {
				require.Len(t, ingresses.Items, 0, "not expected ingresses")
				return
			}

			require.Len(t, ingresses.Items, len(ts.expectedIngress), "not matching expected ingresses count")
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
						continue
					}
					assert.Fail(t, fmt.Sprintf("found not expected ingress %s for: appName %s, host %s, service %s, port %d",
						ingress.GetName(), ingress.GetLabels()[kube.RadixAppLabel], ingress.Spec.Rules[0].Host, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
						&ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number))
				}
			}

		})
	}
}

func registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses []testIngress, config *config.ClusterConfig) error {
	for _, ing := range testIngresses {
		_, err := internal.CreateRadixDNSAliasIngress(kubeClient, ing.appName, ing.envName, internal.BuildRadixDNSAliasIngress(ing.appName, ing.name, ing.component, ing.port, nil, config))
		if err != nil {
			return err
		}
	}
	return nil
}

func getRandomComponentBuilder() utils.RadixApplicationComponentBuilder {
	return utils.NewApplicationComponentBuilder().WithName(commonUtils.RandString(20)).WithPort("p", 9000).WithPublicPort("s")
}

func registerExistingRadixDNSAlias(radixClient radixclient.Interface, radixDNSAlias *radixv1.RadixDNSAlias) error {
	_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.TODO(),
		radixDNSAlias, metav1.CreateOptions{})
	return err
}
