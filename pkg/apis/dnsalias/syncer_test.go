package dnsalias_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	_ "github.com/equinor/radix-operator/pkg/apis/test/initlogger"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
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
	appName     string
	envName     string
	alias       string
	host        string
	component   string
	serviceName string
	port        int32
	labels      map[string]string
}

func (s *syncerTestSuite) Test_OnSync_ingresses() {
	type ingressScenario struct {
		name            string
		dnsAlias        commonTest.DNSAlias
		existingIngress map[string]testIngress
		expectedIngress map[string]testIngress
	}
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

	testDefaultUserGroupID := string(uuid.NewUUID())

	rd1 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd2 := buildRadixDeployment(appName1, component1, component2, envName2, component1Port8080, component2Port9090)
	rd3 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd4 := buildRadixDeployment(appName2, component1, component2, envName2, component1Port8080, component2Port9090)
	scenarios := []ingressScenario{
		{
			name:     "created an ingress",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
		{
			name:     "created additional ingress for another component",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias2.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component2, alias2),
					alias:  alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component2, serviceName: component1, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
				"alias2.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component2, alias2),
					alias:  alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component2, serviceName: component1, port: component1Port8080},
			},
		},
		{
			name:     "changed port changes port in existing ingress",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component2Port9090},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
		{
			name:     "created additional ingress on another alias for the same component",
			dnsAlias: commonTest.DNSAlias{Alias: alias2, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
				"alias2.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias2),
					alias:  alias2, host: dnsalias.GetDNSAliasHost(alias2, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
		{
			name:     "manually changed port repaired",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component2Port9090},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
		{
			name:     "manually changed host repaired",
			dnsAlias: commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: "/manually/edited/host", component: component1, serviceName: component1, port: component1Port8080},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {

			defaultUserGroupID := os.Getenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
			defer s.T().Cleanup(func() {
				if len(defaultUserGroupID) > 0 {
					err := os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, defaultUserGroupID)
					s.Require().NoError(err)
				}
			})
			s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, testDefaultUserGroupID)

			s.SetupTest()
			radixDNSAlias := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Alias, UID: uuid.NewUUID()},
				Spec: radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component}}
			err := commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, ts.dnsAlias.Alias, ts.dnsAlias)
			s.Require().NoError(err, "create existing alias")

			s.registerRadixRegistration(radixDNSAlias.Spec.AppName, testDefaultUserGroupID, nil, nil)
			s.registerRadixDeployments(rd1, rd2, rd3, rd4)
			s.registerExistingIngresses(s.kubeClient, ts.existingIngress)

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
						s.Assert().Equal(expectedIngress.serviceName, service.Name, "rule backend service name")
						s.Assert().Equal(expectedIngress.port, service.Port.Number, "rule backend service port")
						if len(ing.ObjectMeta.OwnerReferences) > 0 {
							ownerRef := ing.ObjectMeta.OwnerReferences[0]
							s.Assert().Equal(radixv1.SchemeGroupVersion.Identifier(), ownerRef.APIVersion, "ownerRef.APIVersion")
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

func (s *syncerTestSuite) Test_OnSync_IngressesWithOAuth2() {
	type ingressScenario struct {
		name               string
		dnsAlias           commonTest.DNSAlias
		existingIngress    map[string]testIngress
		expectedIngress    map[string]testIngress
		componentHasOAuth2 bool
	}
	const (
		appName1           = "app1"
		envName1           = "env1"
		component1         = "component1"
		alias1             = "alias1"
		component1Port8080 = 8080
		dnsZone1           = "dev.radix.equinor.com"
	)

	testDefaultUserGroupID := string(uuid.NewUUID())

	scenarios := []ingressScenario{
		{
			name:               "created an aux ingress",
			componentHasOAuth2: true,
			dnsAlias:           commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
				fmt.Sprintf("alias1.custom-alias-%s", defaults.OAuthProxyAuxiliaryComponentSuffix): {appName: appName1, envName: envName1,
					labels: getLabelsForAuxComponentDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: fmt.Sprintf("%s-%s", component1, defaults.OAuthProxyAuxiliaryComponentSuffix), port: 4180},
			},
		},
		{
			name:               "deleted an aux ingress",
			componentHasOAuth2: false,
			dnsAlias:           commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			existingIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1,
					labels: radixlabels.ForDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
				fmt.Sprintf("alias1.custom-alias-%s", defaults.OAuthProxyAuxiliaryComponentSuffix): {appName: appName1, envName: envName1,
					labels: getLabelsForAuxComponentDNSAliasIngress(appName1, component1, alias1),
					alias:  alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: fmt.Sprintf("%s-%s", component1, defaults.OAuthProxyAuxiliaryComponentSuffix), port: 4180},
			},
			expectedIngress: map[string]testIngress{
				"alias1.custom-alias": {appName: appName1, envName: envName1, alias: alias1, host: dnsalias.GetDNSAliasHost(alias1, dnsZone1), component: component1, serviceName: component1, port: component1Port8080},
			},
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {

			defaultUserGroupID := os.Getenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
			defer s.T().Cleanup(func() {
				if len(defaultUserGroupID) > 0 {
					err := os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, defaultUserGroupID)
					s.Require().NoError(err)
				}
			})
			s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, testDefaultUserGroupID)

			s.SetupTest()
			radixDNSAlias := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Alias, UID: uuid.NewUUID()},
				Spec: radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component}}
			err := commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, ts.dnsAlias.Alias, ts.dnsAlias)
			s.Require().NoError(err, "create existing alias")

			s.registerRadixRegistration(radixDNSAlias.Spec.AppName, testDefaultUserGroupID, nil, nil)
			componentBuilder := utils.NewDeployComponentBuilder().
				WithImage("radixdev.azurecr.io/some-image1:image.tag").
				WithName(component1).
				WithPort("http", component1Port8080).
				WithPublicPort("http")
			if ts.componentHasOAuth2 {
				componentBuilder = componentBuilder.
					WithAuthentication(&radixv1.Authentication{
						OAuth2: &radixv1.OAuth2{
							ClientID: string(uuid.NewUUID()),
						},
					})
			}
			rd := utils.NewDeploymentBuilder().
				WithRadixApplication(utils.ARadixApplication()).
				WithAppName(appName1).
				WithEnvironment(envName1).
				WithComponents(componentBuilder).BuildRD()
			s.registerRadixDeployments(rd)
			s.registerExistingIngresses(s.kubeClient, ts.existingIngress)

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
						if componentName, ok := ing.GetLabels()[kube.RadixAuxiliaryComponentLabel]; ok {
							s.Assert().Equal(expectedIngress.component, componentName, "component name")
						} else {
							s.Assert().Equal(expectedIngress.component, ing.GetLabels()[kube.RadixComponentLabel], "component name")
						}
						s.Assert().Equal(expectedIngress.alias, aliasLabel, "alias name in the label")
						s.Assert().Equal(expectedIngress.host, rule.Host, "rule host")
						s.Assert().Len(rule.IngressRuleValue.HTTP.Paths, 1, "http path count")
						httpIngressPath := rule.IngressRuleValue.HTTP.Paths[0]
						s.Assert().Equal("/", httpIngressPath.Path, "rule http path")
						service := httpIngressPath.Backend.Service
						s.Assert().Equal(expectedIngress.serviceName, service.Name, "rule backend service name")
						s.Assert().Equal(expectedIngress.port, service.Port.Number, "rule backend service port")
						if len(ing.ObjectMeta.OwnerReferences) > 0 {
							ownerRef := ing.ObjectMeta.OwnerReferences[0]
							s.Assert().True(ownerRef.Controller != nil && *ownerRef.Controller, "ownerRef.Controller")
							if _, ok := ing.GetLabels()[kube.RadixAuxiliaryComponentLabel]; ok {
								s.Assert().Equal(networkingv1.SchemeGroupVersion.Identifier(), ownerRef.APIVersion, "ownerRef.APIVersion")
								s.Assert().Equal(k8s.KindIngress, ownerRef.Kind, "ownerRef.Kind")
							} else {
								s.Assert().Equal(radixv1.SchemeGroupVersion.Identifier(), ownerRef.APIVersion, "ownerRef.APIVersion")
								s.Assert().Equal(radixv1.KindRadixDNSAlias, ownerRef.Kind, "ownerRef.Kind")
							}
						}
					}
				}
			}
		})
	}
}

func (s *syncerTestSuite) Test_OnSync_rbac() {
	type testClusterRoleBinding struct {
		adGroups []string
	}
	type rbacScenario struct {
		name                               string
		dnsAlias                           commonTest.DNSAlias
		expectedClusterRoleNames           map[string]any
		expectedClusterRoleBindingSubjects map[string]testClusterRoleBinding
		adminADGroups                      []string
		readerADGroups                     []string
		existingClusterRoleBindings        []string
	}
	const (
		appName1           = "app1"
		appName2           = "app2"
		envName1           = "env1"
		envName2           = "env2"
		component1         = "component1"
		component2         = "component2"
		alias1             = "alias1"
		component1Port8080 = 8080
		component2Port9090 = 9090
	)

	testDefaultUserGroupID := string(uuid.NewUUID())

	rd1 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd2 := buildRadixDeployment(appName1, component1, component2, envName2, component1Port8080, component2Port9090)
	rd3 := buildRadixDeployment(appName1, component1, component2, envName1, component1Port8080, component2Port9090)
	rd4 := buildRadixDeployment(appName2, component1, component2, envName2, component1Port8080, component2Port9090)
	scenarios := []rbacScenario{
		{
			name:           "create rbac for default admin AD group",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  nil,
			readerADGroups: nil,
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1": {adGroups: []string{testDefaultUserGroupID}},
			},
		},
		{
			name:           "create rbac for default admin AD group and reader",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  nil,
			readerADGroups: []string{"b8428b61-a0e6-4e81-af5d-0174e7297733", "cf8d720e-ac1d-42af-8c18-9de0811d81ee"},
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1":        true,
				"radix-platform-user-rda-reader-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1":        {adGroups: []string{testDefaultUserGroupID}},
				"radix-platform-user-rda-reader-app1": {adGroups: []string{"b8428b61-a0e6-4e81-af5d-0174e7297733", "cf8d720e-ac1d-42af-8c18-9de0811d81ee"}},
			},
		},
		{
			name:           "create rbac for specified admin AD group only",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  []string{"bde12869-4a59-490c-bf4d-266ba5f783be", "cca5270d-ffd5-442c-ae32-edb78eee80ce"},
			readerADGroups: nil,
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1": {adGroups: []string{"bde12869-4a59-490c-bf4d-266ba5f783be", "cca5270d-ffd5-442c-ae32-edb78eee80ce"}},
			},
		},
		{
			name:           "create rbac for specified admin AD group and reader group",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  []string{"bde12869-4a59-490c-bf4d-266ba5f783be", "cca5270d-ffd5-442c-ae32-edb78eee80ce"},
			readerADGroups: []string{"b8428b61-a0e6-4e81-af5d-0174e7297733", "cf8d720e-ac1d-42af-8c18-9de0811d81ee"},
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1":        true,
				"radix-platform-user-rda-reader-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1":        {adGroups: []string{"bde12869-4a59-490c-bf4d-266ba5f783be", "cca5270d-ffd5-442c-ae32-edb78eee80ce"}},
				"radix-platform-user-rda-reader-app1": {adGroups: []string{"b8428b61-a0e6-4e81-af5d-0174e7297733", "cf8d720e-ac1d-42af-8c18-9de0811d81ee"}},
			},
		},
		{
			name:           "delete existing reader role binding",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  []string{"bde12869-4a59-490c-bf4d-266ba5f783be"},
			readerADGroups: nil,
			existingClusterRoleBindings: []string{
				"radix-platform-user-rda-app1",
				"radix-platform-user-rda-reader-app1",
			},
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1": {adGroups: []string{"bde12869-4a59-490c-bf4d-266ba5f783be"}},
			},
		},
		{
			name:           "not delete existing admin role binding",
			dnsAlias:       commonTest.DNSAlias{Alias: alias1, Environment: envName1, Component: component1},
			adminADGroups:  nil,
			readerADGroups: nil,
			existingClusterRoleBindings: []string{
				"radix-platform-user-rda-app1",
				"radix-platform-user-rda-reader-app1",
			},
			expectedClusterRoleNames: map[string]any{
				"radix-platform-user-rda-app1": true,
			},
			expectedClusterRoleBindingSubjects: map[string]testClusterRoleBinding{
				"radix-platform-user-rda-app1": {adGroups: []string{testDefaultUserGroupID}},
			},
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			defaultUserGroupID := os.Getenv(defaults.OperatorDefaultUserGroupEnvironmentVariable)
			defer s.T().Cleanup(func() {
				if len(defaultUserGroupID) > 0 {
					err := os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, defaultUserGroupID)
					s.Require().NoError(err)
				}
			})
			s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, testDefaultUserGroupID)

			s.SetupTest()

			ts.dnsAlias.AppName = appName1
			radixDNSAlias := &radixv1.RadixDNSAlias{
				TypeMeta:   metav1.TypeMeta{Kind: radixv1.KindRadixDNSAlias, APIVersion: radixv1.SchemeGroupVersion.Identifier()},
				ObjectMeta: metav1.ObjectMeta{Name: ts.dnsAlias.Alias, UID: uuid.NewUUID(), Labels: radixlabels.ForDNSAliasRbac(appName1)},
				Spec:       radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: ts.dnsAlias.Environment, Component: ts.dnsAlias.Component},
			}
			s.Require().NoError(commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, ts.dnsAlias.Alias, ts.dnsAlias), "create existing alias")

			s.registerRadixRegistration(radixDNSAlias.Spec.AppName, testDefaultUserGroupID, ts.adminADGroups, ts.readerADGroups)
			s.registerRadixDeployments(rd1, rd2, rd3, rd4)
			s.registerClusterRoleBindings(radixDNSAlias, ts.existingClusterRoleBindings)

			syncer := s.createSyncer(radixDNSAlias)
			err := syncer.OnSync()
			s.Assert().NoError(err)

			clusterRoleList, err := s.getClusterRolesForAnyAliases()
			s.Require().NoError(err)

			s.Len(clusterRoleList.Items, len(ts.expectedClusterRoleNames), "not matching expected cluster role count")
			if len(clusterRoleList.Items) == len(ts.expectedClusterRoleNames) {
				for _, role := range clusterRoleList.Items {
					roleName := role.GetName()
					_, ok := ts.expectedClusterRoleNames[roleName]
					s.True(ok, "not found expected role %s", roleName)
					s.Equal(rbacv1.SchemeGroupVersion.Identifier(), role.APIVersion, "invalid api version")
					s.Equal(k8s.KindClusterRole, role.Kind, "invalid kind")
					s.Equal(roleName, role.Name, "invalid name")

					s.Equal(radixDNSAlias.Spec.AppName, role.GetLabels()[kube.RadixAppLabel], "missing or invalid label %s", kube.RadixAppLabel)
					s.Equal("true", role.GetLabels()[kube.RadixAliasLabel], "missing or invalid label %s", kube.RadixAliasLabel)

					s.Len(role.GetOwnerReferences(), 1, "expected one object reference")
					ownerReference := role.GetOwnerReferences()[0]
					s.Equal(radixDNSAlias.GetName(), ownerReference.Name, "invalid owner reference name")
					s.Equal(radixDNSAlias.ObjectMeta.UID, ownerReference.UID, "invalid owner reference uid")
					s.Equal(radixDNSAlias.APIVersion, ownerReference.APIVersion, "invalid owner reference api version")
					s.Equal(radixDNSAlias.Kind, ownerReference.Kind, "invalid owner reference kind")
					s.False(*ownerReference.Controller)

					s.Len(role.Rules, 1, "role should have 1 rule")
					rule := role.Rules[0]
					s.Contains(rule.Verbs, "get", "missing rule verb get")
					s.Contains(rule.Verbs, "list", "missing rule verb list")
					s.Len(rule.APIGroups, 1, "rule should have 1 api group")
					s.Equal(radixv1.SchemeGroupVersion.Group, rule.APIGroups[0], "invalid api group")
					s.Contains(rule.Resources, "radixdnsaliases", "missing rule resource radixdnsalias")
					s.Contains(rule.Resources, "radixdnsaliases/status", "missing rule resource radixdnsalias/status")
					s.Len(rule.ResourceNames, 1, "rule should have 1 resource name")
					s.Contains(rule.ResourceNames, radixDNSAlias.GetName(), "missing resource name %s in the rule", radixDNSAlias.GetName())
				}
			}

			clusterRoleBindingList, err := s.getClusterRolesBindingsForAnyAliases()
			s.Require().NoError(err)
			s.NotNil(clusterRoleBindingList.Items) // TODO

			s.Len(clusterRoleBindingList.Items, len(ts.expectedClusterRoleBindingSubjects), "not matching expected cluster role binding count")
			if len(clusterRoleBindingList.Items) == len(ts.expectedClusterRoleBindingSubjects) {
				for _, roleBinding := range clusterRoleBindingList.Items {
					bindingName := roleBinding.GetName()
					_, ok := ts.expectedClusterRoleNames[bindingName]
					s.True(ok, "not found expected role binding %s", bindingName)
					expectedClusterRoleBinding, ok := ts.expectedClusterRoleBindingSubjects[bindingName]
					s.True(ok, "not found expected role binding subjects")

					s.Len(roleBinding.GetOwnerReferences(), 1, "expected one object reference")
					ownerReference := roleBinding.GetOwnerReferences()[0]
					s.Equal(radixDNSAlias.GetName(), ownerReference.Name, "invalid owner reference name")
					s.Equal(radixDNSAlias.ObjectMeta.UID, ownerReference.UID, "invalid owner reference uid")
					s.Equal(radixDNSAlias.APIVersion, ownerReference.APIVersion, "invalid owner reference api version")
					s.Equal(radixDNSAlias.Kind, ownerReference.Kind, "invalid owner reference kind")
					s.False(*ownerReference.Controller)

					roleRef := roleBinding.RoleRef
					s.Equal(rbacv1.GroupName, roleRef.APIGroup, "invalid api group in role binding role ref")
					s.Equal(k8s.KindClusterRole, roleRef.Kind, "invalid kind in role binding role ref")
					s.Equal(bindingName, roleRef.Name)
					s.Len(roleBinding.Subjects, len(expectedClusterRoleBinding.adGroups), "not matching subjects")
					for _, subject := range roleBinding.Subjects {
						s.Equal(subject.APIGroup, rbacv1.GroupName, "invalid subject api group")
						s.Equal(subject.Kind, rbacv1.GroupKind, "invalid subject kind")
						s.Contains(expectedClusterRoleBinding.adGroups, subject.Name, "missing subject name in expected AD groups")
					}
				}
			}

		})
	}
}

func (s *syncerTestSuite) Test_OnSync_error() {
	const (
		appName1   = "app1"
		envName1   = "env1"
		component1 = "component1"
		alias1     = "alias1"
	)

	scenarios := []struct {
		name          string
		expectedError string
		hasPublicPort bool
	}{
		{
			name:          "error, because the component has no public port",
			hasPublicPort: false,
			expectedError: "component component1 referred to by dnsAlias is not marked as public",
		},
		{
			name:          "no error",
			hasPublicPort: true,
			expectedError: "",
		},
	}
	for _, ts := range scenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			s.SetupTest()

			radixDNSAlias := &radixv1.RadixDNSAlias{
				TypeMeta:   metav1.TypeMeta{Kind: radixv1.KindRadixDNSAlias, APIVersion: radixv1.SchemeGroupVersion.Identifier()},
				ObjectMeta: metav1.ObjectMeta{Name: alias1, UID: uuid.NewUUID(), Labels: radixlabels.ForDNSAliasRbac(appName1)},
				Spec:       radixv1.RadixDNSAliasSpec{AppName: appName1, Environment: envName1, Component: component1},
			}
			s.Require().NoError(commonTest.RegisterRadixDNSAliasBySpec(s.radixClient, alias1, commonTest.DNSAlias{
				Alias: alias1, AppName: appName1, Environment: envName1, Component: component1}), "create existing alias")
			testDefaultUserGroupID := string(uuid.NewUUID())
			s.registerRadixRegistration(radixDNSAlias.Spec.AppName, testDefaultUserGroupID, nil, nil)

			componentBuilder := utils.NewDeployComponentBuilder().WithImage("radixdev.azurecr.io/some-image1:image.tag").WithName(component1).WithPort("http", 8080)
			if ts.hasPublicPort {
				componentBuilder = componentBuilder.WithPublicPort("http")
			}
			rd1 := utils.NewDeploymentBuilder().WithRadixApplication(utils.ARadixApplication()).WithAppName(appName1).
				WithEnvironment(envName1).WithComponents(componentBuilder).BuildRD()
			s.registerRadixDeployments(rd1)

			syncer := s.createSyncer(radixDNSAlias)
			err := syncer.OnSync()

			if len(ts.expectedError) > 0 {
				require.Error(t, err)
				require.EqualError(t, err, ts.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func (s *syncerTestSuite) getIngressesForAnyAliases(namespace string) (*networkingv1.IngressList, error) {
	return s.kubeClient.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: kube.RadixAliasLabel})
}

func (s *syncerTestSuite) getClusterRolesForAnyAliases() (*rbacv1.ClusterRoleList, error) {
	return s.kubeClient.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{LabelSelector: kube.RadixAliasLabel})
}

func (s *syncerTestSuite) getClusterRolesBindingsForAnyAliases() (*rbacv1.ClusterRoleBindingList, error) {
	return s.kubeClient.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{LabelSelector: kube.RadixAliasLabel})
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

func (s *syncerTestSuite) registerRadixRegistration(appName string, defaultAdminADGroup string, adminADGroups []string, readerADGroups []string) {
	if len(adminADGroups) == 0 {
		adminADGroups = []string{defaultAdminADGroup}
	}
	_, err := s.kubeUtil.RadixClient().RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec: radixv1.RadixRegistrationSpec{
			AdGroups:       adminADGroups,
			ReaderAdGroups: readerADGroups,
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err, "create existing radix registration %s", appName)
}

func (s *syncerTestSuite) registerRadixDeployments(radixDeployments ...*radixv1.RadixDeployment) {
	for _, rd := range radixDeployments {
		namespace := utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)
		_, err := s.radixClient.RadixV1().RadixDeployments(namespace).
			Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err, "create existing radix deployment %s", rd.GetName())
	}
}

func (s *syncerTestSuite) registerClusterRoleBindings(radixDNSAlias *radixv1.RadixDNSAlias, roleBindings []string) {
	for _, name := range roleBindings {
		err := s.kubeUtil.ApplyClusterRoleBinding(internal.BuildClusterRoleBinding(name, nil, radixDNSAlias))
		s.Require().NoError(err, "create existing cluster role binding %s", name)
	}
}

func (s *syncerTestSuite) registerExistingIngresses(kubeClient kubernetes.Interface, testIngresses map[string]testIngress) {
	for ingName, ingProps := range testIngresses {
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:   ingName,
				Labels: ingProps.labels,
			},
			Spec: ingress.GetIngressSpec(ingProps.host, ingProps.serviceName, defaults.TLSSecretName, ingProps.port),
		}
		_, err := dnsalias.CreateRadixDNSAliasIngress(kubeClient, ingProps.appName, ingProps.envName, ing)
		s.Require().NoError(err, "create existing ingress %s", ing.GetName())
	}
}

func getLabelsForAuxComponentDNSAliasIngress(appName, componentName, alias string) map[string]string {
	return kubelabels.Set{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     componentName,
		kube.RadixAuxiliaryComponentTypeLabel: defaults.OAuthProxyAuxiliaryComponentType,
		kube.RadixAliasLabel:                  alias,
	}
}
