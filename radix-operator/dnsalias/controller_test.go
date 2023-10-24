package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/stretchr/testify/suite"
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
	sut := dnsalias.NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go sut.Run(5, s.Stop)

	const (
		appName1       = "any-app1"
		appName2       = "any-app2"
		aliasName      = "alias-1"
		envName1       = "env1"
		envName2       = "env2"
		componentName1 = "server1"
		componentName2 = "server2"
	)
	alias := &v1.RadixDNSAlias{ObjectMeta: metav1.ObjectMeta{Name: aliasName,
		Labels: labels.Merge(labels.ForApplicationName(appName1), labels.ForComponentName(componentName1))},
		Spec: v1.RadixDNSAliasSpec{
			AppName:     appName1,
			Environment: envName1,
			Component:   componentName1,
		}}

	// Adding a RadixDNSAlias should trigger sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias, err := s.RadixClient.RadixV1().RadixDNSAliases().Create(context.Background(), alias, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on add RadixDNSAlias")

	// Updating the RadixDNSAlias with appName should trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias.Spec.AppName = appName2
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.TODO(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update appName in the RadixDNSAlias")

	// Updating the RadixDNSAlias with environment should trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias.Spec.Environment = envName2
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.TODO(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update environment in the RadixDNSAlias")

	// Updating the RadixDNSAlias with component should trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias.Spec.Component = componentName2
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.TODO(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update component in the RadixDNSAlias")

	// Updating the RadixDNSAlias with no changes should not trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.TODO(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when updating RadixDNSAlias with no changes")

	// Add Kubernetes Job with ownerreference to RadixDNSAlias should not trigger sync
	ingress := internal.BuildRadixDNSAliasIngress(alias.Spec.AppName, alias.GetName(), alias.Spec.Component, int32(8080))
	ingress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{APIVersion: defaults.RadixAPIVersion, Kind: defaults.RadixDNSAliasKind, Name: aliasName, Controller: pointers.Ptr(true)}}
	namespace := utils.GetEnvironmentNamespace(alias.Spec.AppName, alias.Spec.Environment)
	s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	ingress, err = internal.CreateRadixDNSAliasIngress(s.KubeClient, alias.Spec.AppName, alias.Spec.Environment, ingress)
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding k8s ingress")
	/*
		// Sync should not trigger on ingress update if resource version is unchanged
		s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
		ingress, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), ingress, metav1.UpdateOptions{})
		s.Require().NoError(err)
		s.WaitForNotSynced("Sync should not be called on k8s ingress update with no resource version change")

		// Sync should trigger on ingress update if resource version is changed
		ingress.ResourceVersion = "2"
		s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
		_, err = s.KubeClient.BatchV1().Jobs(namespace).Update(context.Background(), ingress, metav1.UpdateOptions{})
		s.Require().NoError(err)
		s.WaitForSynced("Sync should be called on k8s ingress update with changed resource version")

		// Sync should trigger when deleting ingress
		s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
		err = s.KubeClient.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{})
		s.Require().NoError(err)
		s.WaitForSynced("Sync should be called on k8s ingress deletion")

		// Sync should not trigger when deleting alias
		s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
		err = s.RadixClient.RadixV1().RadixDNSAliases(namespace).Delete(context.Background(), aliasName, metav1.DeleteOptions{})
		s.Require().NoError(err)
		s.WaitForNotSynced("Sync should not be called on alias deletion")

	*/
	// Delete the RadixDNSAlias should trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixDNSAliases().Delete(context.TODO(), alias.GetName(), metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should be called when deleting RadixDNSAlias")

}
