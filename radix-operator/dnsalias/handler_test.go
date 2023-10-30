package dnsalias_test

import (
	"context"
	"fmt"
	"testing"

	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/config"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clusterConfig := &config.ClusterConfig{DNSZone: "test.radix.equinor.com"}
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, clusterConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))

	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), clusterConfig, gomock.Any()).Times(0)
	s.syncer.EXPECT().OnSync().Times(0)

	err := handler.Sync("", domain1, s.EventRecorder)
	s.Require().NoError(err)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsError() {
	clusterConfig := &config.ClusterConfig{DNSZone: "test.radix.equinor.com"}
	s.Require().NoError(registerExistingRadixDNSAliases(s.RadixClient, appName1, env1, component1, domain1), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, clusterConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	expectedError := fmt.Errorf("some error")
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), clusterConfig, gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(expectedError).Times(1)

	actualError := handler.Sync("", domain1, s.EventRecorder)
	s.Equal(expectedError, actualError)
}

func (s *handlerTestSuite) Test_RadixDNSAliases_ReturnsNoError() {
	clusterConfig := &config.ClusterConfig{DNSZone: "test.radix.equinor.com"}
	s.Require().NoError(registerExistingRadixDNSAliases(s.RadixClient, appName1, env1, component1, domain1), "create existing RadixDNSAlias")
	handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient, clusterConfig,
		func(synced bool) {}, dnsalias.WithSyncerFactory(s.syncerFactory))
	s.syncerFactory.EXPECT().CreateSyncer(gomock.Any(), gomock.Any(), gomock.Any(), clusterConfig, gomock.Any()).Return(s.syncer).Times(1)
	s.syncer.EXPECT().OnSync().Return(nil).Times(1)

	err := handler.Sync("", domain1, s.EventRecorder)
	s.Require().Nil(err)
}

func registerExistingRadixDNSAliases(radixClient radixclient.Interface, appName, envName, component, domain string) error {
	_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.Background(),
		&radixv1.RadixDNSAlias{
			ObjectMeta: metav1.ObjectMeta{
				Name:   domain,
				Labels: labels.Merge(labels.ForApplicationName(appName), labels.ForComponentName(component)),
			},
			Spec: radixv1.RadixDNSAliasSpec{
				AppName:     appName,
				Environment: envName,
				Component:   component,
			},
		}, metav1.CreateOptions{})
	return err
}
