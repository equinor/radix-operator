package dnsalias_test

import (
	"fmt"
	"testing"

	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

type handlerTestSuite struct {
	common.ControllerTestSuite
	syncerFactory *internal.MockSyncerFactory
	syncer        *dnsaliasapi.MockSyncer
}

const (
	appName1   = "appName1"
	env1       = "env1"
	component1 = "component1"
	domain1    = "domain1"
)

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
	dnsConfig := &dnsalias2.DNSConfig{DNSZone: "test.radix.equinor.com"}
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, dnsConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))

	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), dnsConfig, gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync().Times(0)

	err := handler.Sync("", domain1, s.EventRecorder)
	s.Require().NoError(err)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsError() {
	dnsConfig := &dnsalias2.DNSConfig{DNSZone: "test.radix.equinor.com"}
	s.Require().NoError(commonTest.RegisterRadixDNSAlias(s.RadixClient, appName1, env1, component1, domain1, 8080), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, dnsConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	expectedError := fmt.Errorf("some error")
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), dnsConfig, gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(expectedError).Times(1)

	actualError := handler.Sync("", domain1, s.EventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsNoError() {
	dnsConfig := &dnsalias2.DNSConfig{DNSZone: "test.radix.equinor.com"}
	s.Require().NoError(commonTest.RegisterRadixDNSAlias(s.RadixClient, appName1, env1, component1, domain1, 8080), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, dnsConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), dnsConfig, gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(nil).Times(1)

	err := handler.Sync("", domain1, s.EventRecorder)
	s.Require().Nil(err)
}
