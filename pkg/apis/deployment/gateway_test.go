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

// Test that HTTPRoute and ListenerSet are created when component is public and has external DNS
func (s *GatewayTestSuite) TestHTTPRouteAndListenerSetCreated_PublicComponentWithExternalDNS() {
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

	// Verify HTTPRoute created
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal(testComponentName, route.Name)
	s.Equal(ns, route.Namespace)

	// Verify hostnames include external DNS, active cluster and cluster-name hosts
	s.GreaterOrEqual(len(route.Spec.Hostnames), 2, "should have at least external DNS and cluster hostnames")
	s.Contains(hostnameStrings(route.Spec.Hostnames), externalDNSFQDN)

	// Verify parent refs include both Gateway and ListenerSet
	s.Len(route.Spec.CommonRouteSpec.ParentRefs, 2, "should have gateway + listenerset parent refs")
	gatewayParent := route.Spec.CommonRouteSpec.ParentRefs[0]
	s.Equal(gatewayapiv1.ObjectName(testGatewayName), gatewayParent.Name)
	s.Equal(gatewayapiv1.Namespace(testGatewayNamespace), *gatewayParent.Namespace)
	s.Equal(gatewayapiv1.SectionName(testGatewaySectionName), *gatewayParent.SectionName)

	lsParent := route.Spec.CommonRouteSpec.ParentRefs[1]
	s.Equal(gatewayapiv1.Group(gatewayapixv1alpha1.GroupName), *lsParent.Group)
	s.Equal(gatewayapiv1.Kind("XListenerSet"), *lsParent.Kind)
	s.Equal(gatewayapiv1.ObjectName(testComponentName), lsParent.Name)
	s.Equal(gatewayapiv1.Namespace(ns), *lsParent.Namespace)

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

	// Verify path match
	s.Require().Len(route.Spec.Rules[0].Matches, 1)
	s.Require().NotNil(route.Spec.Rules[0].Matches[0].Path)
	s.Equal(gatewayapiv1.PathMatchPathPrefix, *route.Spec.Rules[0].Matches[0].Path.Type)
	s.Equal("/", *route.Spec.Rules[0].Matches[0].Path.Value)

	// Verify owner reference
	s.Require().Len(route.OwnerReferences, 1)
	s.Equal(radixv1.KindRadixDeployment, route.OwnerReferences[0].Kind)

	// Verify ListenerSet created for external DNS
	ls, err := s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Equal(testComponentName, ls.Name)

	// Verify listener for external DNS host
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.SectionName(externalDNSFQDN), ls.Spec.Listeners[0].Name)
	s.NotNil(ls.Spec.Listeners[0].Hostname)
	s.Equal(gatewayapixv1alpha1.Hostname(externalDNSFQDN), *ls.Spec.Listeners[0].Hostname)
	s.Equal(gatewayapiv1.HTTPSProtocolType, ls.Spec.Listeners[0].Protocol)
	s.Equal(gatewayapiv1.PortNumber(443), ls.Spec.Listeners[0].Port)

	// Verify TLS config with certificate ref
	s.Require().NotNil(ls.Spec.Listeners[0].TLS)
	s.Equal(gatewayapiv1.TLSModeTerminate, *ls.Spec.Listeners[0].TLS.Mode)
	s.Require().Len(ls.Spec.Listeners[0].TLS.CertificateRefs, 1)
	s.Equal(gatewayapiv1.ObjectName(externalDNSFQDN), ls.Spec.Listeners[0].TLS.CertificateRefs[0].Name)

	// Verify parent gateway reference
	s.Equal(gatewayapiv1.ObjectName(testGatewayName), ls.Spec.ParentRef.Name)
	s.Equal(gatewayapiv1.Namespace(testGatewayNamespace), *ls.Spec.ParentRef.Namespace)

	// Verify labels
	s.Equal(testAppName, ls.Labels[kube.RadixAppLabel])
	s.Equal(testComponentName, ls.Labels[kube.RadixComponentLabel])

	// Verify owner reference
	s.Require().Len(ls.OwnerReferences, 1)
	s.Equal(radixv1.KindRadixDeployment, ls.OwnerReferences[0].Kind)
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

// Test HTTPRoute and ListenerSet are deleted when component is not public anymore
func (s *GatewayTestSuite) TestHTTPRouteAndListenerSetDeleted_WhenComponentNotPublic() {
	externalDNSFQDN := "app.example.com"

	// First deploy with public component + external DNS
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

	// Verify resources exist
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err, "HTTPRoute should exist after first sync")
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err, "ListenerSet should exist after first sync")

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

	// HTTPRoute should be deleted
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute should be deleted when component is not public")

	// ListenerSet should be deleted
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet should be deleted when component is not public")
}

// Test HTTPRoute and ListenerSet are updated when external DNS is changed
func (s *GatewayTestSuite) TestHTTPRouteAndListenerSetUpdated_WhenExternalDNSChanged() {
	originalFQDN := "app.example.com"
	updatedFQDN := "new-app.example.com"

	// Deploy with original external DNS
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: originalFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify original DNS in HTTPRoute and ListenerSet
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Contains(hostnameStrings(route.Spec.Hostnames), originalFQDN)

	ls, err := s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(originalFQDN), *ls.Spec.Listeners[0].Hostname)

	// Deploy with updated external DNS
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: updatedFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// Verify updated DNS in HTTPRoute
	route, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Contains(hostnameStrings(route.Spec.Hostnames), updatedFQDN)
	s.NotContains(hostnameStrings(route.Spec.Hostnames), originalFQDN)

	// Verify updated DNS in ListenerSet
	ls, err = s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err)
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(updatedFQDN), *ls.Spec.Listeners[0].Hostname)
	s.Equal(gatewayapiv1.ObjectName(updatedFQDN), ls.Spec.Listeners[0].TLS.CertificateRefs[0].Name)
}

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

// Test HTTPRoute and ListenerSet are garbage collected when component is removed from spec
func (s *GatewayTestSuite) TestGarbageCollection_WhenComponentRemovedFromSpec() {
	externalDNSFQDN := "app.example.com"

	// Deploy with a public component
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

	// Verify resources exist
	_, err = s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err, "HTTPRoute should exist")
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err, "ListenerSet should exist")

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

	// ListenerSet for removed component should be garbage collected
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet for removed component should be garbage collected")

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

// Test that ListenerSet has multiple listeners when component has multiple external DNS entries
func (s *GatewayTestSuite) TestListenerSetMultipleListeners_MultipleExternalDNS() {
	fqdn1 := "app1.example.com"
	fqdn2 := "app2.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(testAppName).
			WithEnvironment(testEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(testComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(
						radixv1.RadixDeployExternalDNS{FQDN: fqdn1, UseCertificateAutomation: false},
						radixv1.RadixDeployExternalDNS{FQDN: fqdn2, UseCertificateAutomation: false},
					),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	ls, err := s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err)

	// Should have two listeners
	s.Require().Len(ls.Spec.Listeners, 2)
	listenerNames := []string{string(ls.Spec.Listeners[0].Name), string(ls.Spec.Listeners[1].Name)}
	s.ElementsMatch(listenerNames, []string{fqdn1, fqdn2})

	// Verify TLS secret names match FQDNs
	for _, listener := range ls.Spec.Listeners {
		s.Require().NotNil(listener.TLS)
		s.Require().Len(listener.TLS.CertificateRefs, 1)
		s.Equal(gatewayapiv1.ObjectName(*listener.Hostname), listener.TLS.CertificateRefs[0].Name)
	}

	// HTTPRoute should contain all hostnames including external DNS
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)
	routeHostnames := hostnameStrings(route.Spec.Hostnames)
	s.Subset(routeHostnames, []string{fqdn1, fqdn2})
}

// Test ListenerSet is removed when external DNS is removed but component stays public
func (s *GatewayTestSuite) TestListenerSetDeleted_WhenExternalDNSRemoved() {
	externalDNSFQDN := "app.example.com"

	// Deploy with external DNS
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

	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err, "ListenerSet should exist with external DNS")

	// Redeploy without external DNS but still public
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

	// ListenerSet should be deleted
	_, err = s.getListenerSet(ctx, testComponentName, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet should be deleted when external DNS removed")

	// HTTPRoute should still exist (component is still public)
	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err, "HTTPRoute should still exist for public component")
	s.NotContains(hostnameStrings(route.Spec.Hostnames), externalDNSFQDN, "external DNS hostname should be removed from HTTPRoute")

	// HTTPRoute should only have gateway parent ref (no listenerset)
	s.Len(route.Spec.CommonRouteSpec.ParentRefs, 1)
}

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

// Test that all component hostnames are present in HTTPRoute
func (s *GatewayTestSuite) TestHTTPRouteHostnames_ContainsAllDNSTypes() {
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

	// External DNS
	s.Contains(routeHostnames, externalDNSFQDN, "should contain external DNS hostname")

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

// TestLabels_ForComponentGatewayResources verifies correct labels on HTTPRoute and ListenerSet
func (s *GatewayTestSuite) TestLabels_ForComponentGatewayResources() {
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

	route, err := s.getHTTPRoute(ctx, testComponentName, ns)
	s.Require().NoError(err)

	expectedLabels := radixlabels.ForComponentGatewayResources(testAppName, &radixv1.RadixDeployComponent{Name: testComponentName})
	for k, v := range expectedLabels {
		s.Equal(v, route.Labels[k], "HTTPRoute should have label %s=%s", k, v)
	}

	ls, err := s.getListenerSet(ctx, testComponentName, ns)
	s.Require().NoError(err)

	for k, v := range expectedLabels {
		s.Equal(v, ls.Labels[k], "ListenerSet should have label %s=%s", k, v)
	}
}

func hostnameStrings(hostnames []gatewayapiv1.Hostname) []string {
	result := make([]string, len(hostnames))
	for i, h := range hostnames {
		result[i] = string(h)
	}
	return result
}
