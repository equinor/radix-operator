package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
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
	go func() {
		err := sut.Run(5, s.Stop)
		if err != nil {
			s.Require().NoError(err)
		}
	}()

	const (
		appName1       = "any-app1"
		appName2       = "any-app2"
		aliasName      = "alias-alias-1"
		envName1       = "env1"
		envName2       = "env2"
		componentName1 = "server1"
		componentName2 = "server2"
		dnsZone        = "dev.radix.equinor.com"
	)
	alias := internal.BuildRadixDNSAlias(appName1, componentName1, envName1, aliasName)

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

	// Add Ingress with owner reference to RadixDNSAlias should not trigger sync
	cfg := &dnsalias2.DNSConfig{DNSZone: dnsZone}
	ing := buildRadixDNSAliasIngress(alias, int32(8080), cfg)
	ing.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: radix.APIVersion, Kind: radix.KindRadixDNSAlias, Name: aliasName, Controller: pointers.Ptr(true)}})
	namespace := utils.GetEnvironmentNamespace(alias.Spec.AppName, alias.Spec.Environment)
	s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	ing, err = dnsaliasapi.CreateRadixDNSAliasIngress(s.KubeClient, alias.Spec.AppName, alias.Spec.Environment, ing)
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding ingress")

	// Sync should not trigger on ingress update if resource version is unchanged
	s.Handler.EXPECT().Sync(namespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	ing, err = s.KubeClient.NetworkingV1().Ingresses(namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on ingress update with no resource version change")

	// Sync should trigger on ingress update if resource version is changed
	ing.ResourceVersion = "2"
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	_, err = s.KubeClient.NetworkingV1().Ingresses(namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s ingress update with changed resource version")

	// Sync should trigger when deleting ingress
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	err = s.KubeClient.NetworkingV1().Ingresses(namespace).Delete(context.Background(), ing.GetName(), metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on ingress deletion")

	// Delete the RadixDNSAlias should not trigger a sync
	s.Handler.EXPECT().Sync("", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixDNSAliases().Delete(context.TODO(), alias.GetName(), metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should be called when deleting RadixDNSAlias")
}

func buildRadixDNSAliasIngress(dnsAlias *radixv1.RadixDNSAlias, port int32, cfg *dnsalias2.DNSConfig) *networkingv1.Ingress {
	aliasName := dnsAlias.GetName()
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dnsaliasapi.GetDNSAliasIngressName(aliasName),
			Labels: radixlabels.ForDNSAliasIngress(dnsAlias.Spec.AppName, dnsAlias.Spec.Component, aliasName),
		},
		Spec: ingress.GetIngressSpec(dnsaliasapi.GetDNSAliasHost(aliasName, cfg.DNSZone), dnsAlias.Spec.Component, defaults.TLSSecretName, port),
	}
}
