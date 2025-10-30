package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/operator/dnsalias"
	"github.com/equinor/radix-operator/operator/dnsalias/internal"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
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
	alias1     = "alias1"
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
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, "dev.radix.equinor.com",
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))

	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), "dev.radix.equinor.com", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync(gomock.Any()).Times(0)

	err := handler.Sync(context.Background(), "", alias1, s.EventRecorder)
	s.Require().NoError(err)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsError() {
	s.Require().NoError(commonTest.RegisterRadixDNSAlias(context.Background(), s.RadixClient, appName1, env1, component1, alias1), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, "dev.radix.equinor.com",
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	expectedError := fmt.Errorf("some error")
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), "dev.radix.equinor.com", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(expectedError).Times(1)

	actualError := handler.Sync(context.Background(), "", alias1, s.EventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsNoError() {
	s.Require().NoError(commonTest.RegisterRadixDNSAlias(context.Background(), s.RadixClient, appName1, env1, component1, alias1), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, "dev.radix.equinor.com",
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), "dev.radix.equinor.com", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync(gomock.Any()).Return(nil).Times(1)

	err := handler.Sync(context.Background(), "", alias1, s.EventRecorder)
	s.Require().Nil(err)
}
