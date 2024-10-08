package dnsalias_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	dnsalias2 "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	sut := dnsalias.NewController(context.Background(), s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Ctx.Done())
	s.KubeInformerFactory.Start(s.Ctx.Done())
	go func() {
		err := sut.Run(s.Ctx, 5)
		if err != nil {
			s.Require().NoError(err)
		}
	}()

	const (
		appName1       = "any-app1"
		aliasName      = "alias-alias-1"
		aliasName2     = "alias-alias-2"
		envName1       = "env1"
		componentName1 = "server1"
		componentName2 = "server2"
		dnsZone        = "dev.radix.equinor.com"
	)
	rr, err := s.RadixClient.RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: appName1},
		Spec: radixv1.RadixRegistrationSpec{
			AdGroups: []string{string(uuid.NewUUID())},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	alias := internal.BuildRadixDNSAlias(appName1, componentName1, envName1, aliasName)

	// Adding a RadixDNSAlias should trigger sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Create(context.Background(), alias, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on add RadixDNSAlias")

	appNamespace := utils.GetAppNamespace(appName1)
	ra, err := s.RadixClient.RadixV1().RadixApplications(appNamespace).Create(context.Background(), &radixv1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName1},
		Spec:       radixv1.RadixApplicationSpec{DNSAlias: []radixv1.DNSAlias{{Alias: aliasName, Environment: envName1, Component: componentName1}}}}, metav1.CreateOptions{})
	s.Require().NoError(err)

	// Updating the RadixDNSAlias with component should trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alias.Spec.Component = componentName2
	alias, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.Background(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on update component in the RadixDNSAlias")

	// Updating the RadixDNSAlias with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixDNSAliases().Update(context.Background(), alias, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when updating RadixDNSAlias with no changes")

	// Add Ingress with owner reference to RadixDNSAlias should not trigger sync
	cfg := &dnsalias2.DNSConfig{DNSZone: dnsZone}
	ing := buildRadixDNSAliasIngress(alias, int32(8080), cfg)
	ing.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: radixv1.SchemeGroupVersion.Identifier(), Kind: radixv1.KindRadixDNSAlias, Name: aliasName, Controller: pointers.Ptr(true)}})
	envNamespace := utils.GetEnvironmentNamespace(alias.Spec.AppName, alias.Spec.Environment)
	s.Handler.EXPECT().Sync(gomock.Any(), envNamespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	ing, err = dnsaliasapi.CreateRadixDNSAliasIngress(context.Background(), s.KubeClient, alias.Spec.AppName, alias.Spec.Environment, ing)
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called when adding ingress")

	// Sync should not trigger on ingress update if resource version is unchanged
	s.Handler.EXPECT().Sync(gomock.Any(), envNamespace, aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	ing, err = s.KubeClient.NetworkingV1().Ingresses(envNamespace).Update(context.Background(), ing, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called on ingress update with no resource version change")

	// Sync should trigger on ingress update if resource version is changed
	ing.ResourceVersion = "2"
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	_, err = s.KubeClient.NetworkingV1().Ingresses(envNamespace).Update(context.Background(), ing, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on k8s ingress update with changed resource version")

	// Sync should trigger when deleting ingress
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	err = s.KubeClient.NetworkingV1().Ingresses(envNamespace).Delete(context.Background(), ing.GetName(), metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on ingress deletion")

	// Sync should trigger when changed admin adGroup in radix registration
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	rr.Spec.AdGroups = []string{string(uuid.NewUUID())}
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on updated admin ad group")

	// Sync should trigger when changed reader adGroup in radix registration
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	rr.Spec.ReaderAdGroups = []string{string(uuid.NewUUID())}
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on updated reader ad group")

	// Sync should trigger when changed DNSAlias in radix application
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	ra.Spec.DNSAlias[0].Component = componentName2
	ra.ObjectMeta.ResourceVersion = string(uuid.NewUUID())
	_, err = s.RadixClient.RadixV1().RadixApplications(appNamespace).Update(context.Background(), ra, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on updated alias")

	// Sync should trigger when added DNSAlias in radix application
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	ra.Spec.DNSAlias = append(ra.Spec.DNSAlias, radixv1.DNSAlias{
		Alias:       aliasName2,
		Environment: envName1,
		Component:   componentName1,
	})
	ra.ObjectMeta.ResourceVersion = string(uuid.NewUUID())
	_, err = s.RadixClient.RadixV1().RadixApplications(appNamespace).Update(context.Background(), ra, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on added alias")

	// Sync should trigger when deleted DNSAlias in radix application
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	ra.Spec.DNSAlias = ra.Spec.DNSAlias[1:]
	ra.ObjectMeta.ResourceVersion = string(uuid.NewUUID())
	_, err = s.RadixClient.RadixV1().RadixApplications(appNamespace).Update(context.Background(), ra, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called on delete alias")

	// Delete the RadixDNSAlias should not trigger a sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", aliasName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	err = s.RadixClient.RadixV1().RadixDNSAliases().Delete(context.Background(), alias.GetName(), metav1.DeleteOptions{})
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
