package deployment

import (
	"context"
	"fmt"
	"testing"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapixv1alpha1 "sigs.k8s.io/gateway-api/apisx/v1alpha1"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	testGatewayName        = "test-gateway"
	testGatewayNamespace   = "test-gateway-ns"
	testGatewaySectionName = "test-section"
	testDNSZone            = "dev.radix.equinor.com"
	testAppAliasBaseURL    = "app.dev.radix.equinor.com"
	testAppName            = "myapp"
	testEnvName            = "test"
	testComponentName      = "web"
)

type GatewayTestSuite struct {
	suite.Suite
	kubeClient    kubernetes.Interface
	radixClient   radixclient.Interface
	kubeUtil      *kube.Kube
	dynamicClient client.Client
	certClient    *certfake.Clientset
	testUtils     *test.Utils
	cfg           *config.Config
}

func TestGatewayTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayTestSuite))
}

func (s *GatewayTestSuite) SetupSuite() {
	s.T().Setenv(defaults.OperatorDNSZoneEnvironmentVariable, testDNSZone)
	s.T().Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, testAppAliasBaseURL)
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	s.T().Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	s.T().Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	s.T().Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "docker.io/radix-job-scheduler:main-latest")
	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
}

func (s *GatewayTestSuite) SetupTest() {
	s.setupTest()
}

func (s *GatewayTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *GatewayTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	s.radixClient = radixClient
	kedaClient := kedafake.NewSimpleClientset()
	s.dynamicClient = test.CreateClient()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	s.certClient = certfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, radixClient, kedaClient, secretProviderClient)
	handlerTestUtils := test.NewTestUtils(s.kubeClient, radixClient, kedaClient, secretProviderClient)
	s.Require().NoError(handlerTestUtils.CreateClusterPrerequisites(testClusterName, "anysubid"))
	s.testUtils = &handlerTestUtils
	s.cfg = &config.Config{
		DeploymentSyncer:      testConfig.DeploymentSyncer,
		CertificateAutomation: testConfig.CertificateAutomation,
		Gateway: config.GatewayConfig{
			Name:        testGatewayName,
			Namespace:   testGatewayNamespace,
			SectionName: testGatewaySectionName,
		},
	}
}

func (s *GatewayTestSuite) applyDeploymentWithSync(deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	rd, err := s.testUtils.ApplyDeployment(context.Background(), deploymentBuilder)
	if err != nil {
		return nil, err
	}

	rr, err := s.radixClient.RadixV1().RadixRegistrations().Get(context.Background(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	syncer := NewDeploymentSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.dynamicClient, s.certClient, rr, rd, nil, nil, s.cfg)
	if err := syncer.OnSync(context.Background()); err != nil {
		return nil, err
	}

	return s.radixClient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
}

func (s *GatewayTestSuite) applyDeploymentWithSyncAndAnnotations(deploymentBuilder utils.DeploymentBuilder, rrAnnotations map[string]string) (*radixv1.RadixDeployment, error) {
	rd, err := s.testUtils.ApplyDeployment(context.Background(), deploymentBuilder)
	if err != nil {
		return nil, err
	}

	rr, err := s.radixClient.RadixV1().RadixRegistrations().Get(context.Background(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if rrAnnotations != nil {
		rr.Annotations = rrAnnotations
	}

	syncer := NewDeploymentSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.dynamicClient, s.certClient, rr, rd, nil, nil, s.cfg)
	if err := syncer.OnSync(context.Background()); err != nil {
		return nil, err
	}

	return s.radixClient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
}

func (s *GatewayTestSuite) getHTTPRoute(ctx context.Context, name, namespace string) (*gatewayapiv1.HTTPRoute, error) {
	route := &gatewayapiv1.HTTPRoute{}
	err := s.dynamicClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, route)
	return route, err
}

func (s *GatewayTestSuite) listHTTPRoutes(ctx context.Context, namespace string) (*gatewayapiv1.HTTPRouteList, error) {
	routes := &gatewayapiv1.HTTPRouteList{}
	err := s.dynamicClient.List(ctx, routes, client.InNamespace(namespace))
	return routes, err
}

func (s *GatewayTestSuite) getListenerSet(ctx context.Context, name, namespace string) (*gatewayapixv1alpha1.XListenerSet, error) {
	ls := &gatewayapixv1alpha1.XListenerSet{}
	err := s.dynamicClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ls)
	return ls, err
}

func (s *GatewayTestSuite) listListenerSets(ctx context.Context, namespace string) (*gatewayapixv1alpha1.XListenerSetList, error) {
	lsList := &gatewayapixv1alpha1.XListenerSetList{}
	err := s.dynamicClient.List(ctx, lsList, client.InNamespace(namespace))
	return lsList, err
}

func (s *GatewayTestSuite) namespace() string {
	return utils.GetEnvironmentNamespace(testAppName, testEnvName)
}

// Test that component HTTPRoute is created with correct spec when component has external DNS
// External DNS specific HTTPRoute and ListenerSet tests are in externaldns_test.go
func (s *GatewayTestSuite) TestComponentHTTPRoute_PublicComponentWithExternalDNS() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify component HTTPRoute created (named by component, not FQDN)
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal(testComponentName, route.Name)
	s.Equal(ns, route.Namespace)

	// Component HTTPRoute should NOT include external DNS hostname (handled in externaldns.go)
	s.NotContains(hostnameStrings(route.Spec.Hostnames), externalDNSFQDN)
	// Should still have cluster hostnames
	s.Greater(len(route.Spec.Hostnames), 0, "should have cluster hostnames")

	// Component HTTPRoute should only have Gateway parent ref (no ListenerSet)
	s.Require().Len(route.Spec.CommonRouteSpec.ParentRefs, 1, "should only have gateway parent ref")
	gatewayParent := route.Spec.CommonRouteSpec.ParentRefs[0]
	s.Equal(gatewayapiv1.ObjectName(testGatewayName), gatewayParent.Name)
	s.Equal(gatewayapiv1.Namespace(testGatewayNamespace), *gatewayParent.Namespace)
	s.Equal(gatewayapiv1.SectionName(testGatewaySectionName), *gatewayParent.SectionName)

	// Verify labels
	s.Equal(testAppName, route.Labels[kube.RadixAppLabel])
	s.Equal(testComponentName, route.Labels[kube.RadixComponentLabel])

	// Verify backend ref points to the component service
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	s.Equal(gatewayapiv1.ObjectName(testComponentName), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(8080), *route.Spec.Rules[0].BackendRefs[0].Port)

	// Verify HSTS filter
	s.Require().Len(route.Spec.Rules[0].Filters, 1)
	s.Equal(gatewayapiv1.HTTPRouteFilterResponseHeaderModifier, route.Spec.Rules[0].Filters[0].Type)
	s.Require().NotNil(route.Spec.Rules[0].Filters[0].ResponseHeaderModifier)
	s.Require().Len(route.Spec.Rules[0].Filters[0].ResponseHeaderModifier.Add, 1)
	s.Equal(gatewayapiv1.HTTPHeaderName("Strict-Transport-Security"), route.Spec.Rules[0].Filters[0].ResponseHeaderModifier.Add[0].Name)
	s.Equal("max-age=31536000; includeSubDomains", route.Spec.Rules[0].Filters[0].ResponseHeaderModifier.Add[0].Value)

	// Verify path match
	s.Require().Len(route.Spec.Rules[0].Matches, 1)
	s.Require().NotNil(route.Spec.Rules[0].Matches[0].Path)
	s.Equal(gatewayapiv1.PathMatchPathPrefix, *route.Spec.Rules[0].Matches[0].Path.Type)
	s.Equal("/", *route.Spec.Rules[0].Matches[0].Path.Value)

	// Verify owner reference
	s.Require().Len(route.OwnerReferences, 1)
	s.Equal(radixv1.KindRadixDeployment, route.OwnerReferences[0].Kind)
}

// Test that HTTPRoute is created but ListenerSet is NOT created when component is public without external DNS
func (s *GatewayTestSuite) TestHTTPRouteCreated_ListenerSetNotCreated_PublicComponentWithoutExternalDNS() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// HTTPRoute should be created (component is public, so cluster DNS hostnames exist)
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal(testComponentName, route.Name)
	s.Greater(len(route.Spec.Hostnames), 0)

	// Only gateway parent ref (no ListenerSet parent)
	s.Require().Len(route.Spec.CommonRouteSpec.ParentRefs, 1, "should only have gateway parent ref without external DNS")
	gatewayParent := route.Spec.CommonRouteSpec.ParentRefs[0]
	s.Equal(gatewayapiv1.ObjectName(testGatewayName), gatewayParent.Name)

	// ListenerSet should not exist
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet should not exist for public component without external DNS")
}

// Test component HTTPRoute is deleted when component is not public anymore
func (s *GatewayTestSuite) TestHTTPRouteDeleted_WhenComponentNotPublic() {
	// First deploy with public component
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify component HTTPRoute exists
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err, "HTTPRoute should exist after first sync")

	// Now deploy with non-public component (no public port)
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// Component HTTPRoute should be deleted
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute should be deleted when component is not public")
}

// External DNS FQDN change tests are in externaldns_test.go

// Test HTTPRoute and ListenerSet are updated when DnsAppAlias is toggled
func (s *GatewayTestSuite) TestHTTPRouteUpdated_WhenDNSAppAliasChanged() {
	// Deploy with app alias enabled
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSAppAlias(true),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	appAliasFQDN := fmt.Sprintf("%s.%s", testAppName, testAppAliasBaseURL)

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Contains(hostnameStrings(route.Spec.Hostnames), appAliasFQDN, "should have app alias hostname")

	// Deploy with app alias disabled
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSAppAlias(false),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	route, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.NotContains(hostnameStrings(route.Spec.Hostnames), appAliasFQDN, "should not have app alias hostname after disabling")
}

// Test component HTTPRoute is garbage collected when component is removed from spec
func (s *GatewayTestSuite) TestGarbageCollection_WhenComponentRemovedFromSpec() {
	// Deploy with a public component
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify component HTTPRoute exists
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err, "HTTPRoute should exist")

	// Deploy with a different component (removing the original)
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName("other-component").
					WithPort("http", 9090).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// HTTPRoute for removed component should be garbage collected
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute for removed component should be garbage collected")

	// New component's HTTPRoute should exist
	_, err = s.getHTTPRoute(ctx, "other-component", ns)
	s.Require().NoError(err, "HTTPRoute for new component should exist")
}

// Test that OAuth2 backend ref is used when OAuth2 is enabled
func (s *GatewayTestSuite) TestOAuth2BackendRef_WhenOAuth2Enabled() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithAuthentication(&radixv1.Authentication{
						OAuth2: &radixv1.OAuth2{
							ClientID: "client-id",
						},
					}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)

	// Backend ref should point to OAuth proxy service, not the component directly
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	expectedOAuthServiceName := utils.GetAuxOAuthProxyComponentServiceName(testComponentName)
	s.Equal(gatewayapiv1.ObjectName(expectedOAuthServiceName), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(defaults.OAuthProxyPortNumber), *route.Spec.Rules[0].BackendRefs[0].Port)
}

// Test switching from OAuth2 enabled to disabled updates backend ref
func (s *GatewayTestSuite) TestBackendRefUpdated_WhenOAuth2Toggled() {
	// First deploy with OAuth2
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithAuthentication(&radixv1.Authentication{
						OAuth2: &radixv1.OAuth2{
							ClientID: "client-id",
						},
					}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	expectedOAuthServiceName := utils.GetAuxOAuthProxyComponentServiceName(testComponentName)
	s.Equal(gatewayapiv1.ObjectName(expectedOAuthServiceName), route.Spec.Rules[0].BackendRefs[0].Name)

	// Redeploy without OAuth2
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	route, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal(gatewayapiv1.ObjectName(testComponentName), route.Spec.Rules[0].BackendRefs[0].Name)
}

// Multiple external DNS ListenerSet and HTTPRoute tests are in externaldns_test.go

// External DNS resource deletion tests are in externaldns_test.go

// Test that gateway mode annotation is set when gateway API is enabled via RR annotation
func (s *GatewayTestSuite) TestGatewayModeAnnotation_WhenGatewayAPIEnabled() {
	_, err := s.applyDeploymentWithSyncAndAnnotations(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
		map[string]string{annotations.PreviewGatewayModeAnnotation: testEnvName},
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal("true", route.Annotations[annotations.PreviewGatewayModeAnnotation])
	s.Equal("30", route.Annotations["external-dns.alpha.kubernetes.io/ttl"])
}

// Test that gateway mode annotation is NOT set when gateway API is not enabled
func (s *GatewayTestSuite) TestNoGatewayModeAnnotation_WhenGatewayAPINotEnabled() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	_, hasGatewayAnnotation := route.Annotations[annotations.PreviewGatewayModeAnnotation]
	s.False(hasGatewayAnnotation, "should not have gateway mode annotation when not enabled")
	_, hasTTLAnnotation := route.Annotations["external-dns.alpha.kubernetes.io/ttl"]
	s.False(hasTTLAnnotation, "should not have TTL annotation when gateway not enabled")
}

// Test that non-public component has no HTTPRoute or ListenerSet
func (s *GatewayTestSuite) TestNoResources_WhenComponentNotPublic() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	routes, err := s.listHTTPRoutes(ctx, ns)
	s.Require().NoError(err)
	s.Empty(routes.Items, "no HTTPRoutes should exist for non-public component")

	lsList, err := s.listListenerSets(ctx, ns)
	s.Require().NoError(err)
	s.Empty(lsList.Items, "no ListenerSets should exist for non-public component")
}

// Test multiple public components each get their own HTTPRoute
func (s *GatewayTestSuite) TestMultiplePublicComponents_EachGetHTTPRoute() {
	comp1 := "frontend"
	comp2 := "api"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(comp1).
					WithPort("http", 8080).
					WithPublicPort("http"),
				utils.NewDeployComponentBuilder().
					WithName(comp2).
					WithPort("http", 3000).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route1, err := s.getHTTPRoute(ctx, comp1, ns)
	s.Require().NoError(err)
	s.Equal(comp1, route1.Name)
	s.Equal(comp1, route1.Labels[kube.RadixComponentLabel])

	route2, err := s.getHTTPRoute(ctx, comp2, ns)
	s.Require().NoError(err)
	s.Equal(comp2, route2.Name)
	s.Equal(comp2, route2.Labels[kube.RadixComponentLabel])
}

// Test gateway mode annotation removed when gateway API is disabled after being enabled
func (s *GatewayTestSuite) TestGatewayModeAnnotationRemoved_WhenGatewayAPIPreview_IsDisabled() {
	// First deploy with gateway enabled
	_, err := s.applyDeploymentWithSyncAndAnnotations(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
		map[string]string{annotations.PreviewGatewayModeAnnotation: testEnvName},
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal("true", route.Annotations[annotations.PreviewGatewayModeAnnotation])

	// Redeploy without gateway annotation
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	route, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	_, hasGatewayAnnotation := route.Annotations[annotations.PreviewGatewayModeAnnotation]
	s.False(hasGatewayAnnotation, "gateway mode annotation should be removed")
}

// Test that component HTTPRoute contains non-external DNS hostnames
func (s *GatewayTestSuite) TestHTTPRouteHostnames_ContainsNonExternalDNSTypes() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSAppAlias(true).
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)

	routeHostnames := hostnameStrings(route.Spec.Hostnames)

	// App alias
	appAliasFQDN := fmt.Sprintf("%s.%s", testAppName, testAppAliasBaseURL)
	s.Contains(routeHostnames, appAliasFQDN, "should contain app alias hostname")

	// External DNS should NOT be in the component HTTPRoute (handled by externaldns.go)
	s.NotContains(routeHostnames, externalDNSFQDN, "external DNS hostname should not be in component HTTPRoute")

	// Active cluster hostname
	activeClusterHostname := getActiveClusterHostName(testComponentName, ns)
	if activeClusterHostname != "" {
		s.Contains(routeHostnames, activeClusterHostname, "should contain active cluster hostname")
	}

	// Cluster-specific hostname
	clusterHostname := getHostName(testComponentName, ns, testClusterName)
	if clusterHostname != "" {
		s.Contains(routeHostnames, clusterHostname, "should contain cluster-specific hostname")
	}
}

// TestLabels_ForComponentGatewayResources verifies correct labels on component HTTPRoute
func (s *GatewayTestSuite) TestLabels_ForComponentGatewayResources() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)

	expectedLabels := radixlabels.ForComponentGatewayResources(testAppName, &radixv1.RadixDeployComponent{Name: testComponentName})
	for k, v := range expectedLabels {
		s.Equal(v, route.Labels[k], "HTTPRoute should have label %s=%s", k, v)
	}
}

func hostnameStrings(hostnames []gatewayapiv1.Hostname) []string {
	result := make([]string, len(hostnames))
	for i, h := range hostnames {
		result[i] = string(h)
	}
	return result
}
