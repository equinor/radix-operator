package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/operator/dnsalias"
	"github.com/equinor/radix-operator/operator/dnsalias/internal"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) TearDownTest() {
	s.MockCtrl.Finish()
}

func (s *controllerTestSuite) Test_RadixDNSAliasEvents() {
	sut := dnsalias.NewController(context.Background(), s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory)
	s.RadixInformerFactory.Start(s.Ctx.Done())
	s.KubeInformerFactory.Start(s.Ctx.Done())
	go func() {
		err := sut.Run(s.Ctx, 5)
		if err != nil {
			s.Require().NoError(err)
		}
	}()

	s.KubeInformerFactory.WaitForCacheSync(s.Ctx.Done())
	s.RadixInformerFactory.WaitForCacheSync(s.Ctx.Done())

	const (
		appName1       = "any-app1"
		aliasName      = "alias-alias-1"
		aliasName2     = "alias-alias-2"
		envName1       = "env1"
		componentName1 = "server1"
		componentName2 = "server2"
	)
	alias := internal.BuildRadixDNSAlias(appName1, componentName1, envName1, aliasName)

	// Adding a RadixDNSAlias should trigger sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias, err := s.RadixClient.RadixV1().RadixDNSAliases().Create(context.Background(), alias, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on add RadixDNSAlias")

	appNamespace := utils.GetAppNamespace(appName1)
	_, err = s.RadixClient.RadixV1().RadixApplications(appNamespace).Create(context.Background(), &radixv1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName1},
		Spec:       radixv1.RadixApplicationSpec{DNSAlias: []radixv1.DNSAlias{{Alias: aliasName, Environment: envName1, Component: componentName1}}}}, metav1.CreateOptions{})
	s.Require().NoError(err)

	// Updating the RadixDNSAlias with component should trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias.Spec.Component = componentName2
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.Background(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update component in the RadixDNSAlias")

	// Updating the RadixDNSAlias with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.Background(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when updating RadixDNSAlias with no changes")

	// Delete the RadixDNSAlias should not trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixDNSAliases().Delete(context.Background(), alias.GetName(), metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should be called when deleting RadixDNSAlias")
}
