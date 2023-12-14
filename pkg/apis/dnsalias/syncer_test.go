package dnsalias_test

import (
	"context"
	"testing"

	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	dnsConfig     *dnsalias2.DNSConfig
	oauthConfig   defaults.OAuth2Config
	ingressConfig ingress.IngressConfiguration
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.dnsConfig = &dnsalias2.DNSConfig{DNSZone: "dev.radix.equinor.com"}
	s.oauthConfig = defaults.NewOAuth2Config()
	s.ingressConfig = ingress.IngressConfiguration{AnnotationConfigurations: []ingress.AnnotationConfiguration{{Name: "test"}}}

	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
}

func (s *syncerTestSuite) createSyncer(radixDNSAlias *radixv1.RadixDNSAlias) dnsalias.Syncer {
	return dnsalias.NewSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.dnsConfig, s.ingressConfig, s.oauthConfig, ingress.GetAuxOAuthProxyAnnotationProviders(), radixDNSAlias)
}

type testIngress struct {
	appName   string
	envName   string
	alias     string
	host      string
	component string
	port      int32
}

type scenario struct {
	name            string
	dnsAlias        commonTest.DNSAlias
	existingIngress map[string]testIngress
	expectedIngress map[string]testIngress
}

func (s *syncerTestSuite) Test_syncer_OnSync() {
	const (
		appName1           = "app1"
		appName2           = "app2"
		envName1           = "env1"
		envName2           = "env2"
		component1         = "component1"
		component2         = "component2"
		alias1             = "alias1"
		alias2             = "alias2"
		component1Port8080 = 8080
		component2Port9090 = 9090
		dnsZone1           = "dev.radix.equinor.com"
	)
	rd1 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd2 := buildRadixDeployment(appName1, component1, component2, envName2, component1Port8080, component2Port9090)
	rd3 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd4 := buildRadixDeployment(appName2, component1, component2, envName2, component1Port8080, component2Port9090)
	scenarios := []scenario{
		{
			name:     "created an ingress",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
			},
		},
		{
			name:     "created additional ingress for another component",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias2.custom-alias": {appName: appName1, envName: envName1, alias: alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component2, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
				"alias2.custom-alias": {appName: appName1, envName: envName1, alias: alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component2, port: component1Port8080},
			},
		},
		{
			name:     "changed port changes port in existing ingress",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component2Port9090},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
			},
		},
		{
			name:     "created additional ingress on another alias for the same component",
			dnsAlias: commonTest.DNSAlias{Alias: alias2, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
				"alias2.custom-alias": {appName: appName1, envName: envName1, alias: alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component1, port: component1Port8080},
			},
		},
		{
			name:     "manually changed port repaired",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component2Port9090},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
			},
		},
		{
			name:     "manually changed host repaired",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: "/manually/edited/host", component: component1, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, port: component1Port8080},
			},
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()
			radixDNSAlias := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Alias, UID: uuid.NewUUID()},
				Spec: radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component}}
			s.Require().NoError(commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, ts.dnsAlias.Alias, ts.dnsAlias), "create existing alias")

			s.registeringRadixDeployments(rd1, rd2, rd3, rd4)
			err := registerExistingIngresses(s.kubeClient, ts.existingIngress)
			s.Require().NoError(err, "create existing ingresses")

			syncer := s.createSyncer(radixDNSAlias)
			err = syncer.OnSync()
			s.Assert().NoError(err)

			ingresses, err := s.getIngressesForAnyAliases(utils.GetEnvironmentNamespace(appName1, ts.dnsAlias.Environment))
			s.Assert().NoError(err)

			// assert ingresses
			if ts.expectedIngress == nil {
				s.Assert().Len(ingresses.Items, 0, "not expected ingresses")
				return
			}

			s.Len(ingresses.Items, len(ts.expectedIngress), "not matching expected ingresses count")
			if len(ingresses.Items) == len(ts.expectedIngress) {
				for _, ing := range ingresses.Items {
					appNameLabel := ing.GetLabels()[kube.RadixAppLabel]
					componentNameLabel := ing.GetLabels()[kube.RadixComponentLabel]
					aliasLabel := ing.GetLabels()[kube.RadixAliasLabel]
					s.Assert().Len(ing.Spec.Rules, 1, "rules count")
					rule := ing.Spec.Rules[0]
					expectedIngress, ingressExists := ts.expectedIngress[ing.Name]
					assert.True(t, ingressExists, "found not expected ingress %s for: appName %s, host %s, service %s, port %d",
						ing.GetName(), appNameLabel, rule.Host, rule.HTTP.Paths[0].Backend.Service.Name,
						rule.HTTP.Paths[0].Backend.Service.Port.Number)
					if ingressExists {
						s.Assert().Equal(expectedIngress.appName, appNameLabel, "app name")
						expectedNamespace := utils.GetEnvironmentNamespace(expectedIngress.appName, expectedIngress.envName)
						s.Assert().Equal(expectedNamespace, ing.GetNamespace(), "namespace")
						s.Assert().Equal(expectedIngress.component, componentNameLabel, "component name")
						s.Assert().Equal(expectedIngress.alias, aliasLabel, "alias name in the label")
						s.Assert().Equal(expectedIngress.host, rule.Host, "rule host")
						s.Assert().Len(rule.IngressRuleValue.HTTP.Paths, 1, "http path count")
						httpIngressPath := rule.IngressRuleValue.HTTP.Paths[0]
						s.Assert().Equal("/", httpIngressPath.Path, "rule http path")
						service := httpIngressPath.Backend.Service
						s.Assert().Equal(expectedIngress.component, service.Name, "rule backend service name")
						s.Assert().Equal(expectedIngress.port, service.Port.Number, "rule backend service port")
						if len(ing.ObjectMeta.OwnerReferences) > 0 {
							ownerRef := ing.ObjectMeta.OwnerReferences[0]
							s.Assert().Equal(radixv1.APIVersion, ownerRef.APIVersion, "ownerRef.APIVersion")
							s.Assert().Equal(radixv1.KindRadixDNSAlias, ownerRef.Kind, "ownerRef.Kind")
							s.Assert().Equal(radixDNSAlias.GetName(), ownerRef.Name, "ownerRef.Name")
							s.Assert().Equal(radixDNSAlias.GetUID(), ownerRef.UID, "ownerRef.UID")
							s.Assert().True(ownerRef.Controller != nil && *ownerRef.Controller, "ownerRef.Controller")
						}
					}
				}
			}

		})
	}
}

func (s *syncerTestSuite) getIngressesForAnyAliases(namespace string) (*networkingv1.IngressList, error) {
	return s.kubeClient.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: kube.RadixAliasLabel})
}

func buildRadixDeployment(appName, component1, component2, envName string, port8080, port9090 int32) *radixv1.RadixDeployment {
	return utils.NewDeploymentBuilder().
		WithRadixApplication(utils.ARadixApplication()).
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(utils.NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/some-image1:image.tag").
			WithName(component1).
			WithPort("http", port8080).
			WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/some-image2:image.tag").
				WithName(component2).
				WithPort("http", port9090).
				WithPublicPort("http")).BuildRD()
}

func (s *syncerTestSuite) registeringRadixDeployments(radixDeployments ...*radixv1.RadixDeployment) {
	for _, rd := range radixDeployments {
		namespace := utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)
		_, err := s.radixClient.RadixV1().RadixDeployments(namespace).
			Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
	}
}

func registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses map[string]testIngress) error {
	for _, ingProps := range testIngresses {
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:   dnsalias.GetDNSAliasIngressName(ingProps.alias),
				Labels: radixlabels.ForDNSAliasIngress(ingProps.appName, ingProps.component, ingProps.alias),
			},
			Spec: ingress.GetIngressSpec(ingProps.host, ingProps.component, defaults.TLSSecretName, ingProps.port),
		}
		_, err := dnsalias.CreateRadixDNSAliasIngress(kubeClient, ingProps.appName, ingProps.envName, ing)
		if err != nil {
			return err
		}
	}
	return nil
}
