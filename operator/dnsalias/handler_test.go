package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/operator/dnsalias"
	"github.com/equinor/radix-operator/operator/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type handlerTestSuite struct {
	common.ControllerTestSuite
	syncerFactory *internal.MockSyncerFactory
	syncer        *dnsaliasapi.MockSyncer
}

func (s *handlerTestSuite) SetupTest() {
	s.ControllerTestSuite.SetupTest()
	s.syncerFactory = internal.NewMockSyncerFactory(s.MockCtrl)
	s.syncer = dnsaliasapi.NewMockSyncer(s.MockCtrl)
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}

func (s *handlerTestSuite) TearDownTest() {
	s.MockCtrl.Finish()
}

func (s *handlerTestSuite) Test_RadixDNSAliases_NotFound() {
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.EventRecorder, "dev.radix.equinor.com", dnsalias.WithSyncerFactory(s.syncerFactory))

	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync(gomock.Any()).Times(0)

	err := handler.Sync(context.Background(), "", "any")
	s.Require().NoError(err)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsError() {
	expectedDnsZone := "any.zone.com"
	expectedOauth2Cfg := defaults.NewMockOAuth2Config(s.MockCtrl)
	expectedDnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: v1.ObjectMeta{Name: "any-dns-alias"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     "any-app",
			Component:   "any-component",
			Environment: "any-env",
		},
	}
	expectedDnsAlias, err := s.RadixClient.RadixV1().RadixDNSAliases().Create(context.Background(), expectedDnsAlias, v1.CreateOptions{})
	expectedTagretNamespace := utils.GetEnvironmentNamespace(expectedDnsAlias.Spec.AppName, expectedDnsAlias.Spec.Environment)
	expectedComponentIngressAnnotations := ingress.GetComponentAnnotationProvider(ingress.IngressConfiguration{}, expectedTagretNamespace, expectedOauth2Cfg)
	expectedOAuthIngressAnnotations := ingress.GetOAuthAnnotationProviders()
	expectedOAuthProxyModeIngressAnnotations := ingress.GetOAuthProxyModeAnnotationProviders(ingress.IngressConfiguration{}, expectedTagretNamespace)
	expectedError := fmt.Errorf("some error")
	s.Require().NoError(err)
	s.syncerFactory.EXPECT().CreateSyncer(expectedDnsAlias, s.KubeClient, s.KubeUtil, s.RadixClient, expectedDnsZone, expectedOauth2Cfg, expectedComponentIngressAnnotations, expectedOAuthIngressAnnotations, expectedOAuthProxyModeIngressAnnotations).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(expectedError).Times(1)

	sut := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.EventRecorder, expectedDnsZone, dnsalias.WithSyncerFactory(s.syncerFactory), dnsalias.WithOAuth2DefaultConfig(expectedOauth2Cfg))
	actualError := sut.Sync(context.Background(), "", expectedDnsAlias.Name)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsNoError() {
	expectedDnsZone := "any.zone.com"
	expectedOauth2Cfg := defaults.NewMockOAuth2Config(s.MockCtrl)
	expectedDnsAlias := &radixv1.RadixDNSAlias{
		ObjectMeta: v1.ObjectMeta{Name: "any-dns-alias"},
		Spec: radixv1.RadixDNSAliasSpec{
			AppName:     "any-app",
			Component:   "any-component",
			Environment: "any-env",
		},
	}
	expectedDnsAlias, err := s.RadixClient.RadixV1().RadixDNSAliases().Create(context.Background(), expectedDnsAlias, v1.CreateOptions{})
	expectedTagretNamespace := utils.GetEnvironmentNamespace(expectedDnsAlias.Spec.AppName, expectedDnsAlias.Spec.Environment)
	expectedComponentIngressAnnotations := ingress.GetComponentAnnotationProvider(ingress.IngressConfiguration{}, expectedTagretNamespace, expectedOauth2Cfg)
	expectedOAuthIngressAnnotations := ingress.GetOAuthAnnotationProviders()
	expectedOAuthProxyModeIngressAnnotations := ingress.GetOAuthProxyModeAnnotationProviders(ingress.IngressConfiguration{}, expectedTagretNamespace)
	s.Require().NoError(err)
	s.syncerFactory.EXPECT().CreateSyncer(expectedDnsAlias, s.KubeClient, s.KubeUtil, s.RadixClient, expectedDnsZone, expectedOauth2Cfg, expectedComponentIngressAnnotations, expectedOAuthIngressAnnotations, expectedOAuthProxyModeIngressAnnotations).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(nil).Times(1)

	sut := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.EventRecorder, expectedDnsZone, dnsalias.WithSyncerFactory(s.syncerFactory), dnsalias.WithOAuth2DefaultConfig(expectedOauth2Cfg))
	s.NoError(sut.Sync(context.Background(), "", expectedDnsAlias.Name))
}
