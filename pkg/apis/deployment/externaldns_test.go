package deployment

import (
	"context"
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
	edTestGatewayName      = "test-gateway"
	edTestGatewayNamespace = "test-gateway-ns"
	edTestAppName          = "myapp"
	edTestEnvName          = "test"
	edTestComponentName    = "web"
)

type ExternalDNSTestSuite struct {
	suite.Suite
	kubeClient    kubernetes.Interface
	radixClient   radixclient.Interface
	kubeUtil      *kube.Kube
	dynamicClient client.Client
	certClient    *certfake.Clientset
	testUtils     *test.Utils
	cfg           *config.Config
}

func TestExternalDNSTestSuite(t *testing.T) {
	suite.Run(t, new(ExternalDNSTestSuite))
}

func (s *ExternalDNSTestSuite) SetupSuite() {
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

func (s *ExternalDNSTestSuite) SetupTest() {
	s.setupTest()
}

func (s *ExternalDNSTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *ExternalDNSTestSuite) setupTest() {
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
			Name:      edTestGatewayName,
			Namespace: edTestGatewayNamespace,
		},
	}
}

func (s *ExternalDNSTestSuite) applyDeploymentWithSync(deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
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

func (s *ExternalDNSTestSuite) applyDeploymentWithSyncAndAnnotations(deploymentBuilder utils.DeploymentBuilder, rrAnnotations map[string]string) (*radixv1.RadixDeployment, error) {
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

func (s *ExternalDNSTestSuite) getHTTPRoute(ctx context.Context, name, namespace string) (*gatewayapiv1.HTTPRoute, error) {
	route := &gatewayapiv1.HTTPRoute{}
	err := s.dynamicClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, route)
	return route, err
}

func (s *ExternalDNSTestSuite) listHTTPRoutes(ctx context.Context, namespace string) (*gatewayapiv1.HTTPRouteList, error) {
	routes := &gatewayapiv1.HTTPRouteList{}
	err := s.dynamicClient.List(ctx, routes, client.InNamespace(namespace))
	return routes, err
}

func (s *ExternalDNSTestSuite) getListenerSet(ctx context.Context, name, namespace string) (*gatewayapixv1alpha1.XListenerSet, error) {
	ls := &gatewayapixv1alpha1.XListenerSet{}
	err := s.dynamicClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ls)
	return ls, err
}

func (s *ExternalDNSTestSuite) listListenerSets(ctx context.Context, namespace string) (*gatewayapixv1alpha1.XListenerSetList, error) {
	lsList := &gatewayapixv1alpha1.XListenerSetList{}
	err := s.dynamicClient.List(ctx, lsList, client.InNamespace(namespace))
	return lsList, err
}

func (s *ExternalDNSTestSuite) namespace() string {
	return utils.GetEnvironmentNamespace(edTestAppName, edTestEnvName)
}

// Test that FQDN-named HTTPRoute and ListenerSet are created for each external DNS entry
func (s *ExternalDNSTestSuite) TestHTTPRouteAndListenerSetCreated_ForExternalDNS() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify FQDN-named HTTPRoute created
	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Equal(externalDNSFQDN, route.Name)
	s.Equal(ns, route.Namespace)

	// Verify hostname is the FQDN
	s.Require().Len(route.Spec.Hostnames, 1)
	s.Equal(gatewayapiv1.Hostname(externalDNSFQDN), route.Spec.Hostnames[0])

	// Verify parent ref points to the ListenerSet (not the Gateway)
	s.Require().Len(route.Spec.CommonRouteSpec.ParentRefs, 1)
	parentRef := route.Spec.CommonRouteSpec.ParentRefs[0]
	s.Equal(gatewayapiv1.Group(gatewayapixv1alpha1.GroupName), *parentRef.Group)
	s.Equal(gatewayapiv1.Kind("XListenerSet"), *parentRef.Kind)
	s.Equal(gatewayapiv1.ObjectName(externalDNSFQDN), parentRef.Name)
	s.Equal(gatewayapiv1.Namespace(ns), *parentRef.Namespace)

	// Verify backend ref points to the component service
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	s.Equal(gatewayapiv1.ObjectName(edTestComponentName), route.Spec.Rules[0].BackendRefs[0].Name)
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

	// Verify FQDN-named ListenerSet created
	ls, err := s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Equal(externalDNSFQDN, ls.Name)

	// Verify listener configuration
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.SectionName("https"), ls.Spec.Listeners[0].Name)
	s.NotNil(ls.Spec.Listeners[0].Hostname)
	s.Equal(gatewayapixv1alpha1.Hostname(externalDNSFQDN), *ls.Spec.Listeners[0].Hostname)
	s.Equal(gatewayapiv1.HTTPSProtocolType, ls.Spec.Listeners[0].Protocol)
	s.Equal(gatewayapiv1.PortNumber(443), ls.Spec.Listeners[0].Port)

	// Verify TLS config with certificate ref
	s.Require().NotNil(ls.Spec.Listeners[0].TLS)
	s.Equal(gatewayapiv1.TLSModeTerminate, *ls.Spec.Listeners[0].TLS.Mode)
	s.Require().Len(ls.Spec.Listeners[0].TLS.CertificateRefs, 1)
	expectedSecretName := utils.GetExternalDnsTlsSecretName(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN})
	s.Equal(gatewayapiv1.ObjectName(expectedSecretName), ls.Spec.Listeners[0].TLS.CertificateRefs[0].Name)

	// Verify allowed routes from same namespace
	s.Require().NotNil(ls.Spec.Listeners[0].AllowedRoutes)
	s.Require().NotNil(ls.Spec.Listeners[0].AllowedRoutes.Namespaces)
	s.Equal(gatewayapiv1.NamespacesFromSame, *ls.Spec.Listeners[0].AllowedRoutes.Namespaces.From)

	// Verify parent gateway reference
	s.Equal(gatewayapiv1.ObjectName(edTestGatewayName), ls.Spec.ParentRef.Name)
	s.Equal(gatewayapiv1.Namespace(edTestGatewayNamespace), *ls.Spec.ParentRef.Namespace)

	// Verify owner reference
	s.Require().Len(ls.OwnerReferences, 1)
	s.Equal(radixv1.KindRadixDeployment, ls.OwnerReferences[0].Kind)
}

// Test that labels are correctly set on external DNS HTTPRoute and ListenerSet
func (s *ExternalDNSTestSuite) TestLabels_ForExternalDNSResources() {
	externalDNSFQDN := "app.example.com"
	ed := radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(ed),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	expectedLabels := radixlabels.ForExternalDNSResource(edTestAppName, ed)

	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	for k, v := range expectedLabels {
		s.Equal(v, route.Labels[k], "HTTPRoute should have label %s=%s", k, v)
	}

	ls, err := s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	for k, v := range expectedLabels {
		s.Equal(v, ls.Labels[k], "ListenerSet should have label %s=%s", k, v)
	}
}

// Test that each external DNS entry gets its own FQDN-named HTTPRoute and ListenerSet
func (s *ExternalDNSTestSuite) TestMultipleExternalDNS_EachGetOwnResources() {
	fqdn1 := "app1.example.com"
	fqdn2 := "app2.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
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

	// Each FQDN should have its own ListenerSet
	ls1, err := s.getListenerSet(ctx, fqdn1, ns)
	s.Require().NoError(err)
	s.Equal(fqdn1, ls1.Name)
	s.Require().Len(ls1.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(fqdn1), *ls1.Spec.Listeners[0].Hostname)

	ls2, err := s.getListenerSet(ctx, fqdn2, ns)
	s.Require().NoError(err)
	s.Equal(fqdn2, ls2.Name)
	s.Require().Len(ls2.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(fqdn2), *ls2.Spec.Listeners[0].Hostname)

	// Each FQDN should have its own HTTPRoute
	route1, err := s.getHTTPRoute(ctx, fqdn1, ns)
	s.Require().NoError(err)
	s.Require().Len(route1.Spec.Hostnames, 1)
	s.Equal(gatewayapiv1.Hostname(fqdn1), route1.Spec.Hostnames[0])

	route2, err := s.getHTTPRoute(ctx, fqdn2, ns)
	s.Require().NoError(err)
	s.Require().Len(route2.Spec.Hostnames, 1)
	s.Equal(gatewayapiv1.Hostname(fqdn2), route2.Spec.Hostnames[0])

	// Verify TLS secret names match FQDNs
	expectedSecret1 := utils.GetExternalDnsTlsSecretName(radixv1.RadixDeployExternalDNS{FQDN: fqdn1})
	s.Equal(gatewayapiv1.ObjectName(expectedSecret1), ls1.Spec.Listeners[0].TLS.CertificateRefs[0].Name)

	expectedSecret2 := utils.GetExternalDnsTlsSecretName(radixv1.RadixDeployExternalDNS{FQDN: fqdn2})
	s.Equal(gatewayapiv1.ObjectName(expectedSecret2), ls2.Spec.Listeners[0].TLS.CertificateRefs[0].Name)
}

// Test that old FQDN resources are garbage collected and new ones are created when external DNS FQDN changes
func (s *ExternalDNSTestSuite) TestResourcesUpdated_WhenExternalDNSChanged() {
	originalFQDN := "app.example.com"
	updatedFQDN := "new-app.example.com"

	// Deploy with original external DNS
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: originalFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify original FQDN resources exist
	_, err = s.getListenerSet(ctx, originalFQDN, ns)
	s.Require().NoError(err, "ListenerSet for original FQDN should exist")
	_, err = s.getHTTPRoute(ctx, originalFQDN, ns)
	s.Require().NoError(err, "HTTPRoute for original FQDN should exist")

	// Deploy with updated external DNS
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: updatedFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// Verify original FQDN resources are garbage collected
	_, err = s.getListenerSet(ctx, originalFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet for original FQDN should be deleted")
	_, err = s.getHTTPRoute(ctx, originalFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute for original FQDN should be deleted")

	// Verify new FQDN resources are created
	ls, err := s.getListenerSet(ctx, updatedFQDN, ns)
	s.Require().NoError(err, "ListenerSet for updated FQDN should exist")
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(updatedFQDN), *ls.Spec.Listeners[0].Hostname)

	route, err := s.getHTTPRoute(ctx, updatedFQDN, ns)
	s.Require().NoError(err, "HTTPRoute for updated FQDN should exist")
	s.Require().Len(route.Spec.Hostnames, 1)
	s.Equal(gatewayapiv1.Hostname(updatedFQDN), route.Spec.Hostnames[0])
}

// Test that external DNS resources are deleted when external DNS is removed but component stays public
func (s *ExternalDNSTestSuite) TestResourcesDeleted_WhenExternalDNSRemoved() {
	externalDNSFQDN := "app.example.com"

	// Deploy with external DNS
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify external DNS resources exist
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "ListenerSet should exist with external DNS")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "HTTPRoute should exist with external DNS")

	// Redeploy without external DNS but still public
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// External DNS resources should be deleted
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet should be deleted when external DNS removed")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute should be deleted when external DNS removed")

	// Component HTTPRoute should still exist (component is still public)
	_, err = s.getHTTPRoute(ctx, edTestComponentName, ns)
	s.Require().NoError(err, "Component HTTPRoute should still exist for public component")
}

// Test that external DNS resources are deleted when component becomes non-public
func (s *ExternalDNSTestSuite) TestResourcesDeleted_WhenComponentNotPublic() {
	externalDNSFQDN := "app.example.com"

	// Deploy with public component + external DNS
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
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
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "ListenerSet should exist after first sync")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "HTTPRoute should exist after first sync")

	// Deploy with non-public component (no public port, no external DNS)
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// External DNS resources should be deleted
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet should be deleted when component is not public")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute should be deleted when component is not public")
}

// Test that external DNS resources are garbage collected when the component is removed from spec
func (s *ExternalDNSTestSuite) TestGarbageCollection_WhenComponentRemovedFromSpec() {
	externalDNSFQDN := "app.example.com"

	// Deploy with a public component + external DNS
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
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
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "ListenerSet should exist")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err, "HTTPRoute should exist")

	// Deploy with a different component (removing the original)
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName("other-component").
					WithPort("http", 9090).
					WithPublicPort("http"),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// External DNS resources for removed component should be garbage collected
	_, err = s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet for removed component should be garbage collected")
	_, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute for removed component should be garbage collected")
}

// Test that only the removed external DNS entry is garbage collected while others remain
func (s *ExternalDNSTestSuite) TestPartialGarbageCollection_WhenOneExternalDNSRemoved() {
	fqdn1 := "app1.example.com"
	fqdn2 := "app2.example.com"

	// Deploy with two external DNS entries
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
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

	// Verify both exist
	_, err = s.getListenerSet(ctx, fqdn1, ns)
	s.Require().NoError(err, "ListenerSet for fqdn1 should exist")
	_, err = s.getHTTPRoute(ctx, fqdn1, ns)
	s.Require().NoError(err, "HTTPRoute for fqdn1 should exist")
	_, err = s.getListenerSet(ctx, fqdn2, ns)
	s.Require().NoError(err, "ListenerSet for fqdn2 should exist")
	_, err = s.getHTTPRoute(ctx, fqdn2, ns)
	s.Require().NoError(err, "HTTPRoute for fqdn2 should exist")

	// Redeploy with only fqdn1 (removing fqdn2)
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn1, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// fqdn1 resources should still exist
	_, err = s.getListenerSet(ctx, fqdn1, ns)
	s.Require().NoError(err, "ListenerSet for fqdn1 should still exist")
	_, err = s.getHTTPRoute(ctx, fqdn1, ns)
	s.Require().NoError(err, "HTTPRoute for fqdn1 should still exist")

	// fqdn2 resources should be garbage collected
	_, err = s.getListenerSet(ctx, fqdn2, ns)
	s.True(k8serrors.IsNotFound(err), "ListenerSet for fqdn2 should be garbage collected")
	_, err = s.getHTTPRoute(ctx, fqdn2, ns)
	s.True(k8serrors.IsNotFound(err), "HTTPRoute for fqdn2 should be garbage collected")
}

// Test that non-public component has no external DNS resources
func (s *ExternalDNSTestSuite) TestNoResources_WhenComponentNotPublic() {
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	lsList, err := s.listListenerSets(ctx, ns)
	s.Require().NoError(err)
	s.Empty(lsList.Items, "no ListenerSets should exist for non-public component")
}

// Test that backend ref is updated when external DNS is moved from one component to another
func (s *ExternalDNSTestSuite) TestBackendRefUpdated_WhenExternalDNSMovedBetweenComponents() {
	externalDNSFQDN := "app.example.com"
	comp1 := "frontend"
	comp2 := "api"

	// Deploy with external DNS on comp1
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(comp1).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
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

	// Verify backend ref points to comp1
	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	s.Equal(gatewayapiv1.ObjectName(comp1), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(8080), *route.Spec.Rules[0].BackendRefs[0].Port)

	// Redeploy with external DNS moved to comp2
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(comp1).
					WithPort("http", 8080).
					WithPublicPort("http"),
				utils.NewDeployComponentBuilder().
					WithName(comp2).
					WithPort("http", 3000).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// Verify backend ref now points to comp2
	route, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	s.Equal(gatewayapiv1.ObjectName(comp2), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(3000), *route.Spec.Rules[0].BackendRefs[0].Port)

	// ListenerSet should still exist and be unchanged
	ls, err := s.getListenerSet(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Require().Len(ls.Spec.Listeners, 1)
	s.Equal(gatewayapixv1alpha1.Hostname(externalDNSFQDN), *ls.Spec.Listeners[0].Hostname)
}

// Test that backend ref switches from OAuth2 proxy to direct component service when OAuth2 is removed
func (s *ExternalDNSTestSuite) TestBackendRefUpdated_WhenOAuth2Toggled() {
	externalDNSFQDN := "app.example.com"

	// Deploy with OAuth2 enabled
	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}).
					WithAuthentication(&radixv1.Authentication{
						OAuth2: &radixv1.OAuth2{ClientID: "client-id"},
					}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	// Verify backend points to OAuth proxy
	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	expectedOAuthServiceName := utils.GetAuxOAuthProxyComponentServiceName(edTestComponentName)
	s.Equal(gatewayapiv1.ObjectName(expectedOAuthServiceName), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(defaults.OAuthProxyPortNumber), *route.Spec.Rules[0].BackendRefs[0].Port)

	// Redeploy without OAuth2
	_, err = s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	// Verify backend now points directly to component
	route, err = s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Equal(gatewayapiv1.ObjectName(edTestComponentName), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(8080), *route.Spec.Rules[0].BackendRefs[0].Port)
}

// Test that OAuth2 backend ref is used on external DNS HTTPRoute when OAuth2 is enabled
func (s *ExternalDNSTestSuite) TestOAuth2BackendRef_WhenOAuth2Enabled() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}).
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

	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)

	// Backend ref should point to OAuth proxy service
	s.Require().Len(route.Spec.Rules, 1)
	s.Require().Len(route.Spec.Rules[0].BackendRefs, 1)
	expectedOAuthServiceName := utils.GetAuxOAuthProxyComponentServiceName(edTestComponentName)
	s.Equal(gatewayapiv1.ObjectName(expectedOAuthServiceName), route.Spec.Rules[0].BackendRefs[0].Name)
	s.Equal(gatewayapiv1.PortNumber(defaults.OAuthProxyPortNumber), *route.Spec.Rules[0].BackendRefs[0].Port)
}

// Test that gateway mode annotation is set on external DNS HTTPRoute when gateway API is enabled
func (s *ExternalDNSTestSuite) TestGatewayModeAnnotation_WhenGatewayAPIEnabled() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSyncAndAnnotations(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
		map[string]string{annotations.PreviewGatewayModeAnnotation: edTestEnvName},
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	s.Equal("true", route.Annotations[annotations.PreviewGatewayModeAnnotation])
	s.Equal("30", route.Annotations["external-dns.alpha.kubernetes.io/ttl"])
}

// Test that gateway mode annotation is NOT set on external DNS HTTPRoute when gateway API is not enabled
func (s *ExternalDNSTestSuite) TestNoGatewayModeAnnotation_WhenGatewayAPINotEnabled() {
	externalDNSFQDN := "app.example.com"

	_, err := s.applyDeploymentWithSync(
		utils.ARadixDeployment().
			WithAppName(edTestAppName).
			WithEnvironment(edTestEnvName).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(edTestComponentName).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: externalDNSFQDN, UseCertificateAutomation: false}),
			).
			WithJobComponents(),
	)
	s.Require().NoError(err)

	ctx := context.Background()
	ns := s.namespace()

	route, err := s.getHTTPRoute(ctx, externalDNSFQDN, ns)
	s.Require().NoError(err)
	_, hasGatewayAnnotation := route.Annotations[annotations.PreviewGatewayModeAnnotation]
	s.False(hasGatewayAnnotation, "should not have gateway mode annotation when not enabled")
	_, hasTTLAnnotation := route.Annotations["external-dns.alpha.kubernetes.io/ttl"]
	s.False(hasTTLAnnotation, "should not have TTL annotation when gateway not enabled")
}
