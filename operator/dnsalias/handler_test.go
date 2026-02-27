package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/operator/dnsalias"
	"github.com/equinor/radix-operator/operator/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
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
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.DynamicClient, s.EventRecorder, config.Config{}, dnsalias.WithSyncerFactory(s.syncerFactory))

	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync(gomock.Any()).Times(0)

	err := handler.Sync(context.Background(), "", "any")
	s.Require().NoError(err)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsError() {
	c := config.Config{}
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
	s.syncerFactory.EXPECT().CreateSyncer(expectedDnsAlias, s.KubeClient, s.KubeUtil, s.RadixClient, s.DynamicClient, c, expectedOauth2Cfg, expectedComponentIngressAnnotations, expectedOAuthIngressAnnotations, expectedOAuthProxyModeIngressAnnotations).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(expectedError).Times(1)

	sut := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.DynamicClient, s.EventRecorder, c, dnsalias.WithSyncerFactory(s.syncerFactory), dnsalias.WithOAuth2DefaultConfig(expectedOauth2Cfg))
	actualError := sut.Sync(context.Background(), "", expectedDnsAlias.Name)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsNoError() {
	c := config.Config{
		DNSZone: "any.zone.com",
	}
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
	s.syncerFactory.EXPECT().CreateSyncer(expectedDnsAlias, s.KubeClient, s.KubeUtil, s.RadixClient, s.DynamicClient, c, expectedOauth2Cfg, expectedComponentIngressAnnotations, expectedOAuthIngressAnnotations, expectedOAuthProxyModeIngressAnnotations).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(nil).Times(1)

	sut := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, s.DynamicClient, s.EventRecorder, c, dnsalias.WithSyncerFactory(s.syncerFactory), dnsalias.WithOAuth2DefaultConfig(expectedOauth2Cfg))
	s.NoError(sut.Sync(context.Background(), "", expectedDnsAlias.Name))
}
