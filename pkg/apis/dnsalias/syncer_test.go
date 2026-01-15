package dnsalias_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	gomock "github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type syncerTestSuite struct {
	suite.Suite
	kubeClient                      *kubefake.Clientset
	radixClient                     *radixfake.Clientset
	testUtils                       commontest.Utils
	promClient                      *prometheusfake.Clientset
	dnsZone                         string
	ctrl                            *gomock.Controller
	oauthConfig                     *defaults.MockOAuth2Config
	componentIngressAnnotation      *ingress.MockAnnotationProvider
	oauthIngressAnnotation          *ingress.MockAnnotationProvider
	oauthProxyModeIngressAnnotation *ingress.MockAnnotationProvider
}

func TestSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(syncerTestSuite))
}

func (s *syncerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *syncerTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *syncerTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.testUtils = commontest.NewTestUtils(s.kubeClient, s.radixClient, nil, nil)
	s.dnsZone = "dev.radix.equinor.com"
	s.ctrl = gomock.NewController(s.T())
	s.oauthConfig = defaults.NewMockOAuth2Config(s.ctrl)
	s.componentIngressAnnotation = ingress.NewMockAnnotationProvider(s.ctrl)
	s.oauthIngressAnnotation = ingress.NewMockAnnotationProvider(s.ctrl)
	s.oauthProxyModeIngressAnnotation = ingress.NewMockAnnotationProvider(s.ctrl)
}

func (s *syncerTestSuite) createSyncer(radixDNSAlias *radixv1.RadixDNSAlias) dnsalias.Syncer {
	return dnsalias.NewSyncer(
		radixDNSAlias,
		s.kubeClient,
		s.testUtils.GetKubeUtil(),
		s.radixClient,
		s.dnsZone,
		s.oauthConfig,
		[]ingress.AnnotationProvider{s.componentIngressAnnotation},
		[]ingress.AnnotationProvider{s.oauthIngressAnnotation},
		[]ingress.AnnotationProvider{s.oauthProxyModeIngressAnnotation},
	)
}

func (s *syncerTestSuite) Test_OnSync_ReconcileStatus() {
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: "app"}}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	s.Require().NoError(err)
	rda := &radixv1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: "any-name", Generation: 42}, Spec: radixv1.RadixDNSAliasSpec{AppName: "app"}}
	rda, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), rda, metav1.CreateOptions{})
	s.Require().NoError(err)

	// First sync sets status
	expectedGen := rda.Generation
	sut := dnsalias.NewSyncer(rda, s.kubeClient, s.testUtils.GetKubeUtil(), s.radixClient, s.dnsZone, s.oauthConfig, nil, nil, nil)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	rda, err = s.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), rda.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixDNSAliasReconcileSucceeded, rda.Status.ReconcileStatus)
	s.Empty(rda.Status.Message)
	s.Equal(expectedGen, rda.Status.ObservedGeneration)
	s.False(rda.Status.Reconciled.IsZero())

	// Second sync with updated generation
	rda.Generation++
	expectedGen = rda.Generation
	sut = dnsalias.NewSyncer(rda, s.kubeClient, s.testUtils.GetKubeUtil(), s.radixClient, s.dnsZone, s.oauthConfig, nil, nil, nil)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	rda, err = s.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), rda.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixDNSAliasReconcileSucceeded, rda.Status.ReconcileStatus)
	s.Empty(rda.Status.Message)
	s.Equal(expectedGen, rda.Status.ObservedGeneration)
	s.False(rda.Status.Reconciled.IsZero())

	// Sync with error
	errorMsg := "any sync error"
	s.kubeClient.PrependReactor("*", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New(errorMsg)
	})
	rda.Generation++
	expectedGen = rda.Generation
	sut = dnsalias.NewSyncer(rda, s.kubeClient, s.testUtils.GetKubeUtil(), s.radixClient, s.dnsZone, s.oauthConfig, nil, nil, nil)
	err = sut.OnSync(context.Background())
	s.Require().ErrorContains(err, errorMsg)
	rda, err = s.radixClient.RadixV1().RadixDNSAliases().Get(context.Background(), rda.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixDNSAliasReconcileFailed, rda.Status.ReconcileStatus)
	s.Contains(rda.Status.Message, errorMsg)
	s.Equal(expectedGen, rda.Status.ObservedGeneration)
	s.False(rda.Status.Reconciled.IsZero())
}

func (s *syncerTestSuite) Test_OnSync_Component_IngressSpec() {
	const (
		aliasName     = "any-alias"
		appName       = "any-app"
		envName       = "any-env"
		componentName = "any-comp"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	rrBuilder := utils.NewRegistrationBuilder().WithName(appName)
	_, err := s.testUtils.ApplyRegistration(rrBuilder)
	s.Require().NoError(err)

	rdBuilder := utils.NewDeploymentBuilder().
		WithDeploymentName("any-rd").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName).
				WithPort("metrics", 1000).
				WithPort("http", 8000).
				WithPublicPort("http"),
		)
	rd, err := s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
	s.Require().NoError(err)

	dnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: aliasName},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   componentName,
		},
	}
	dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
	s.Require().NoError(err)

	expectedIngressAnnotations := map[string]string{"any-annotation-key": "any-annotation-value"}
	s.componentIngressAnnotation.EXPECT().GetAnnotations(&rd.Spec.Components[0]).Times(1).Return(expectedIngressAnnotations, nil)

	sut := s.createSyncer(dnsAlias)
	s.Require().NoError(sut.OnSync(context.Background()))

	ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedIngressNames := []string{fmt.Sprintf("%s.custom-alias", aliasName)}
	actualIngressNames := slice.Map(ingresses.Items, func(ing networkingv1.Ingress) string { return ing.Name })
	s.Require().ElementsMatch(expectedIngressNames, actualIngressNames)

	ing := ingresses.Items[0]

	expectedLabels := map[string]string(labels.ForDNSAliasComponentIngress(dnsAlias))
	s.Equal(expectedLabels, ing.Labels)

	s.Equal(expectedIngressAnnotations, ing.Annotations)

	expectedOwnerReferences := []metav1.OwnerReference{{
		APIVersion:         radixv1.SchemeGroupVersion.String(),
		Kind:               "RadixDNSAlias",
		Name:               aliasName,
		Controller:         pointers.Ptr(true),
		BlockOwnerDeletion: pointers.Ptr(true),
	}}
	s.ElementsMatch(expectedOwnerReferences, ing.OwnerReferences)

	expectedHostName := fmt.Sprintf("%s.%s", aliasName, s.dnsZone)
	expectedIngressSpec := ingress.BuildIngressSpecForComponent(&rd.Spec.Components[0], expectedHostName, "")
	s.Equal(expectedIngressSpec, ing.Spec)
}

func (s *syncerTestSuite) Test_OnSync_Component_ChangeDNSAliasComponent_IngressSpec_() {
	const (
		aliasName      = "any-alias"
		appName        = "any-app"
		envName        = "any-env"
		component1Name = "any-comp1"
		component2Name = "any-comp2"
	)
	var (
		envNamespace = utils.GetEnvironmentNamespace(appName, envName)
	)

	rrBuilder := utils.NewRegistrationBuilder().WithName(appName)
	_, err := s.testUtils.ApplyRegistration(rrBuilder)
	s.Require().NoError(err)

	rdBuilder := utils.NewDeploymentBuilder().
		WithDeploymentName("any-rd").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(component1Name).WithPort("http", 8000).WithPublicPort("http"),
			utils.NewDeployComponentBuilder().WithName(component2Name).WithPort("http", 9000).WithPublicPort("http"),
		)
	rd, err := s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
	s.Require().NoError(err)

	dnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: aliasName},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   component1Name,
		},
	}
	dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
	s.Require().NoError(err)

	// Initial sync (create initial ingress)
	s.componentIngressAnnotation.EXPECT().GetAnnotations(&rd.Spec.Components[0]).Times(1).Return(nil, nil)
	sut := s.createSyncer(dnsAlias)
	s.Require().NoError(sut.OnSync(context.Background()))
	ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(ingresses.Items, 1)

	// Change DNSAlias component
	dnsAlias.Spec.Component = component2Name
	dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Update(context.Background(), dnsAlias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.componentIngressAnnotation.EXPECT().GetAnnotations(&rd.Spec.Components[1]).Times(1).Return(nil, nil)
	sut = s.createSyncer(dnsAlias)
	s.Require().NoError(sut.OnSync(context.Background()))

	ingresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(ingresses.Items, 1)

	ing := ingresses.Items[0]

	expectedHostName := fmt.Sprintf("%s.%s", aliasName, s.dnsZone)
	expectedIngressSpec := ingress.BuildIngressSpecForComponent(&rd.Spec.Components[1], expectedHostName, "")
	s.Equal(expectedIngressSpec, ing.Spec)
}

func (s *syncerTestSuite) Test_OnSync_ComponentWithOAuth2_IngressSpec() {
	const (
		aliasName     = "any-alias"
		appName       = "any-app"
		envName       = "any-env"
		componentName = "any-comp"
	)
	var (
		envNamespace     = utils.GetEnvironmentNamespace(appName, envName)
		compIngressName  = fmt.Sprintf("%s.custom-alias", aliasName)
		oauthIngressName = fmt.Sprintf("%s.custom-alias-aux-oauth", aliasName)
	)

	rrBuilder := utils.NewRegistrationBuilder().WithName(appName)
	_, err := s.testUtils.ApplyRegistration(rrBuilder)
	s.Require().NoError(err)

	rdBuilder := utils.NewDeploymentBuilder().
		WithDeploymentName("any-rd").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName).
				WithPort("metrics", 1000).
				WithPort("http", 8000).
				WithPublicPort("http").
				WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "any-client-id"}}),
		)
	rd, err := s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
	s.Require().NoError(err)

	dnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: aliasName},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   componentName,
		},
	}
	dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
	s.Require().NoError(err)

	expectedOAuth := &radixv1.OAuth2{ProxyPrefix: "/any/oauth/path"}
	s.oauthConfig.EXPECT().MergeWith(rd.Spec.Components[0].Authentication.OAuth2).Times(1).Return(expectedOAuth, nil)
	expectedComponent := rd.Spec.Components[0].DeepCopy()
	expectedComponent.Authentication.OAuth2 = expectedOAuth
	expectedCompIngressAnnotations := map[string]string{"any-comp-annotation-key": "any-comp-annotation-value"}
	s.componentIngressAnnotation.EXPECT().GetAnnotations(expectedComponent).Times(1).Return(expectedCompIngressAnnotations, nil)
	expectedOAuth2IngressAnnotations := map[string]string{"any-oauth2-annotation-key": "any-oauth2-annotation-value"}
	s.oauthIngressAnnotation.EXPECT().GetAnnotations(expectedComponent).Times(1).Return(expectedOAuth2IngressAnnotations, nil)

	sut := s.createSyncer(dnsAlias)
	s.Require().NoError(sut.OnSync(context.Background()))

	ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedIngressNames := []string{compIngressName, oauthIngressName}
	actualIngressNames := slice.Map(ingresses.Items, func(ing networkingv1.Ingress) string { return ing.Name })
	s.Require().ElementsMatch(expectedIngressNames, actualIngressNames)

	expectedOwnerReferences := []metav1.OwnerReference{{
		APIVersion:         radixv1.SchemeGroupVersion.String(),
		Kind:               "RadixDNSAlias",
		Name:               aliasName,
		Controller:         pointers.Ptr(true),
		BlockOwnerDeletion: pointers.Ptr(true),
	}}
	expectedLabels := map[string]string(labels.ForDNSAliasComponentIngress(dnsAlias))
	expectedHostName := fmt.Sprintf("%s.%s", aliasName, s.dnsZone)

	compIngress, _ := slice.FindFirst(ingresses.Items, func(ing networkingv1.Ingress) bool { return ing.Name == compIngressName })
	s.Equal(expectedLabels, compIngress.Labels)
	s.Equal(expectedCompIngressAnnotations, compIngress.Annotations)
	s.ElementsMatch(expectedOwnerReferences, compIngress.OwnerReferences)
	expectedIngressSpec := ingress.BuildIngressSpecForComponent(&rd.Spec.Components[0], expectedHostName, "")
	s.Equal(expectedIngressSpec, compIngress.Spec)

	oauthIngress, _ := slice.FindFirst(ingresses.Items, func(ing networkingv1.Ingress) bool { return ing.Name == oauthIngressName })
	s.Equal(expectedLabels, oauthIngress.Labels)
	s.Equal(expectedOAuth2IngressAnnotations, oauthIngress.Annotations)
	s.ElementsMatch(expectedOwnerReferences, oauthIngress.OwnerReferences)
	expectedIngressSpec = ingress.BuildIngressSpecForOAuth2Component(expectedComponent, expectedHostName, "", false)
	s.Equal(expectedIngressSpec, oauthIngress.Spec)
}

func (s *syncerTestSuite) Test_OnSync_ComponentWithOAuth2_ProxyMode_IngressSpec() {
	const (
		aliasName     = "any-alias"
		appName       = "any-app"
		envName       = "any-env"
		componentName = "any-comp"
	)
	var (
		envNamespace     = utils.GetEnvironmentNamespace(appName, envName)
		oauthIngressName = fmt.Sprintf("%s.custom-alias-aux-oauth", aliasName)
	)

	tests := map[string]struct {
		rdAnnotations map[string]string
		rrAnnotations map[string]string
	}{
		"rd proxy mode enabled": {
			rdAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName},
			rrAnnotations: map[string]string{},
		},
		"rd proxy mode enabled, rr enabled for other env": {
			rdAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName},
			rrAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "other"},
		},
		"rr proxy mode enabled": {
			rdAnnotations: map[string]string{},
			rrAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName},
		},
		"rr proxy mode enabled, rd enabled for other env": {
			rdAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "other"},
			rrAnnotations: map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {

			rrBuilder := utils.NewRegistrationBuilder().WithName(appName).WithAnnotations(test.rrAnnotations)
			_, err := s.testUtils.ApplyRegistration(rrBuilder)
			s.Require().NoError(err)

			rdBuilder := utils.NewDeploymentBuilder().
				WithAnnotations(test.rdAnnotations).
				WithDeploymentName("any-rd").
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponents(
					utils.NewDeployComponentBuilder().
						WithName(componentName).
						WithPort("metrics", 1000).
						WithPort("http", 8000).
						WithPublicPort("http").
						WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "any-client-id"}}),
				)
			rd, err := s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
			s.Require().NoError(err)

			dnsAlias := &radixv1.RadixDNSAlias{
				ObjectMeta: metav1.ObjectMeta{Name: aliasName},
				Spec: radixv1.RadixDNSAliasSpec{
					AppName:     appName,
					Environment: envName,
					Component:   componentName,
				},
			}
			dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
			s.Require().NoError(err)

			expectedOAuth := &radixv1.OAuth2{ProxyPrefix: "/any/oauth/path"}
			s.oauthConfig.EXPECT().MergeWith(rd.Spec.Components[0].Authentication.OAuth2).Times(1).Return(expectedOAuth, nil)
			expectedComponent := rd.Spec.Components[0].DeepCopy()
			expectedComponent.Authentication.OAuth2 = expectedOAuth
			expectedOAuth2IngressAnnotations := map[string]string{"any-oauth2-annotation-key": "any-oauth2-annotation-value"}
			s.oauthProxyModeIngressAnnotation.EXPECT().GetAnnotations(expectedComponent).Times(1).Return(expectedOAuth2IngressAnnotations, nil)

			sut := s.createSyncer(dnsAlias)
			s.Require().NoError(sut.OnSync(context.Background()))

			ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			expectedIngressNames := []string{oauthIngressName}
			actualIngressNames := slice.Map(ingresses.Items, func(ing networkingv1.Ingress) string { return ing.Name })
			s.Require().ElementsMatch(expectedIngressNames, actualIngressNames)

			expectedOwnerReferences := []metav1.OwnerReference{{
				APIVersion:         radixv1.SchemeGroupVersion.String(),
				Kind:               "RadixDNSAlias",
				Name:               aliasName,
				Controller:         pointers.Ptr(true),
				BlockOwnerDeletion: pointers.Ptr(true),
			}}
			expectedLabels := map[string]string(labels.ForDNSAliasComponentIngress(dnsAlias))
			expectedHostName := fmt.Sprintf("%s.%s", aliasName, s.dnsZone)

			oauthIngress, _ := slice.FindFirst(ingresses.Items, func(ing networkingv1.Ingress) bool { return ing.Name == oauthIngressName })
			s.Equal(expectedLabels, oauthIngress.Labels)
			s.Equal(expectedOAuth2IngressAnnotations, oauthIngress.Annotations)
			s.ElementsMatch(expectedOwnerReferences, oauthIngress.OwnerReferences)
			expectedIngressSpec := ingress.BuildIngressSpecForOAuth2Component(expectedComponent, expectedHostName, "", true)
			s.Equal(expectedIngressSpec, oauthIngress.Spec)
		})
	}

}

/*
Errors:
  - rr does not exist
*/

func (s *syncerTestSuite) Test_OnSync_GarbageCollect_Ingresses() {
	const (
		aliasName     = "any-alias"
		appName       = "any-app"
		envName       = "any-env"
		componentName = "any-comp"
	)
	var (
		envNamespace     = utils.GetEnvironmentNamespace(appName, envName)
		compIngressName  = fmt.Sprintf("%s.custom-alias", aliasName)
		oauthIngressName = fmt.Sprintf("%s.custom-alias-aux-oauth", aliasName)
	)

	tests := map[string]struct {
		expectedIngressNames []string
		rdBuilderFactory     func(rd utils.DeploymentBuilder) utils.DeploymentBuilder
		rrBuilderMutator     func(rr utils.RegistrationBuilder)
	}{
		"no changes": {
			expectedIngressNames: []string{compIngressName, oauthIngressName},
		},
		"no oauth": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithComponents(utils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8000).WithPublicPort("http"))
			},
			expectedIngressNames: []string{compIngressName},
		},
		"component not public": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithComponents(utils.NewDeployComponentBuilder().WithName(componentName).WithPort("http", 8000))
			},
			expectedIngressNames: []string{},
		},
		"component does not exist": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithComponents()
			},
			expectedIngressNames: []string{},
		},
		"rd status not active": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithCondition(radixv1.DeploymentInactive)
			},
			expectedIngressNames: []string{},
		},
		"proxy mode enabled for current env on rd, other env on rr": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithAnnotations(map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName})
			},
			rrBuilderMutator: func(rr utils.RegistrationBuilder) {
				rr.WithAnnotations(map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "other"})
			},
			expectedIngressNames: []string{oauthIngressName},
		},
		"proxy mode enabled for other env on rd, current env on rr": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return rd.WithAnnotations(map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: "other"})
			},
			rrBuilderMutator: func(rr utils.RegistrationBuilder) {
				rr.WithAnnotations(map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName})
			},
			expectedIngressNames: []string{oauthIngressName},
		},
		"no rd exist": {
			rdBuilderFactory: func(rd utils.DeploymentBuilder) utils.DeploymentBuilder {
				return nil
			},
			expectedIngressNames: []string{},
		},
	}

	for testName, test := range tests {
		s.Run(testName, func() {
			// Setup fixture - component with oauth2
			rrBuilder := utils.NewRegistrationBuilder().WithName(appName)
			rr, err := s.testUtils.ApplyRegistration(rrBuilder)
			s.Require().NoError(err)

			rdBuilder := utils.NewDeploymentBuilder().
				WithDeploymentName("any-rd").
				WithAppName(appName).
				WithEnvironment(envName).
				WithComponents(
					utils.NewDeployComponentBuilder().
						WithName(componentName).
						WithPort("metrics", 1000).
						WithPort("http", 8000).
						WithPublicPort("http").
						WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "any-client-id"}}),
				)
			rd, err := s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
			s.Require().NoError(err)

			dnsAlias := &radixv1.RadixDNSAlias{
				ObjectMeta: metav1.ObjectMeta{Name: aliasName},
				Spec: radixv1.RadixDNSAliasSpec{
					AppName:     appName,
					Environment: envName,
					Component:   componentName,
				},
			}
			dnsAlias, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
			s.Require().NoError(err)

			s.oauthConfig.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{ProxyPrefix: "/any"}, nil)
			s.componentIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, nil)
			s.oauthIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, nil)
			s.oauthProxyModeIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, nil)

			sut := s.createSyncer(dnsAlias)
			s.Require().NoError(sut.OnSync(context.Background()))

			ingresses, _ := s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(ingresses.Items, 2)

			// Run test
			if test.rdBuilderFactory != nil {
				err = s.radixClient.RadixV1().RadixDeployments(envNamespace).Delete(context.Background(), rd.Name, metav1.DeleteOptions{})
				s.Require().NoError(err)
				rdBuilder = test.rdBuilderFactory(rdBuilder)

				if !commonutils.IsNil(rdBuilder) {
					_, err = s.testUtils.ApplyDeployment(context.Background(), rdBuilder)
					s.Require().NoError(err)
				}
			}

			if test.rrBuilderMutator != nil {
				err = s.radixClient.RadixV1().RadixRegistrations().Delete(context.Background(), rr.Name, metav1.DeleteOptions{})
				s.Require().NoError(err)
				test.rrBuilderMutator(rrBuilder)
				_, err = s.testUtils.ApplyRegistration(rrBuilder)
				s.Require().NoError(err)
			}

			sut = s.createSyncer(dnsAlias)
			s.Require().NoError(sut.OnSync(context.Background()))

			ingresses, _ = s.kubeClient.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
			actualIngressNames := slice.Map(ingresses.Items, func(ing networkingv1.Ingress) string { return ing.Name })
			s.ElementsMatch(test.expectedIngressNames, actualIngressNames)
		})
	}

}

func (s *syncerTestSuite) Test_OnSync_Errors() {
	const (
		appName  = "any-app"
		envName  = "any-env"
		compName = "any-comp"
	)

	dnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: metav1.ObjectMeta{Name: "any-name"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     appName,
			Environment: envName,
			Component:   compName,
		},
	}
	rr := utils.NewRegistrationBuilder().
		WithName(appName).
		BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithDeploymentName("any-rd").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().
			WithName(compName).
			WithPort("http", 8000).
			WithPublicPort("http").
			WithAuthentication(&radixv1.Authentication{OAuth2: &radixv1.OAuth2{}})).
		BuildRD()

	s.Run("missing rr", func() {
		_, err := s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
		s.Require().NoError(err)
		sut := s.createSyncer(dnsAlias)
		err = sut.OnSync(context.Background())
		s.Require().Error(err)
		s.True(k8sErrors.IsNotFound(err))
	})

	s.Run("oauth2Config.MergeWith returns error", func() {
		_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
		s.Require().NoError(err)

		sut := s.createSyncer(dnsAlias)
		expectedError := errors.New("any error")
		s.oauthConfig.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(nil, expectedError)
		err = sut.OnSync(context.Background())
		s.Require().ErrorIs(err, expectedError)
	})

	s.Run("component ingress annotation returns error", func() {
		_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
		s.Require().NoError(err)

		sut := s.createSyncer(dnsAlias)
		s.oauthConfig.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
		expectedError := errors.New("any error")
		s.componentIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, expectedError)
		err = sut.OnSync(context.Background())
		s.Require().ErrorIs(err, expectedError)
	})

	s.Run("oauth ingress annotation returns error", func() {
		_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
		s.Require().NoError(err)

		sut := s.createSyncer(dnsAlias)
		s.oauthConfig.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
		s.componentIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, nil)
		expectedError := errors.New("any error")
		s.oauthIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, expectedError)
		err = sut.OnSync(context.Background())
		s.Require().ErrorIs(err, expectedError)
	})

	s.Run("oauth proxy mode ingress annotation returns error", func() {
		rr := rr.DeepCopy()
		rr.Annotations = map[string]string{annotations.PreviewOAuth2ProxyModeAnnotation: envName}
		_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
		s.Require().NoError(err)
		_, err = s.radixClient.RadixV1().RadixDNSAliases().Create(context.Background(), dnsAlias, metav1.CreateOptions{})
		s.Require().NoError(err)

		sut := s.createSyncer(dnsAlias)
		s.oauthConfig.EXPECT().MergeWith(gomock.Any()).AnyTimes().Return(&radixv1.OAuth2{}, nil)
		expectedError := errors.New("any error")
		s.oauthProxyModeIngressAnnotation.EXPECT().GetAnnotations(gomock.Any()).AnyTimes().Return(nil, expectedError)
		err = sut.OnSync(context.Background())
		s.Require().ErrorIs(err, expectedError)
	})
}
