package dnsalias_test

import (
	"context"
	"testing"

	dnsaliasapi "github.com/equinor/radix-operator/pkg/apis/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/equinor/radix-operator/radix-operator/dnsalias"
	"github.com/equinor/radix-operator/radix-operator/dnsalias/internal"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (s *handlerTestSuite) Test_IngressesForRadixDNSAliases() {
	const (
		appName        = "any-app1"
		env1           = "env1"
		componentPort1 = 8080
		component1     = "component1"
	)
	var testScenarios = []struct {
		name                    string
		dnsAlias                radixv1.DNSAlias
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
	}{
		{
			name:     "new alias, no existing RDA, no existing ingresses, additional radix aliases, additional ingresses",
			dnsAlias: radixv1.DNSAlias{Domain: "domain1", Environment: env1, Component: component1},
		},
	}

	for _, ts := range testScenarios {
		s.T().Run(ts.name, func(t *testing.T) {
			// s.SetupTest()
			ra := utils.ARadixApplication().WithAppName(appName).
				WithEnvironment(ts.dnsAlias.Environment, "branch1").
				WithComponent(utils.NewApplicationComponentBuilder().WithName(ts.dnsAlias.Component).WithPort("http", componentPort1)).BuildRA()
			_, err := s.RadixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), ra, metav1.CreateOptions{})
			require.NoError(t, err)
			require.NoError(t, registerExistingRadixDNSAliases(s.RadixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")

			// TODO
			require.NoError(t, registerExistingRadixDNSAliases(s.RadixClient,
				map[string]radixv1.RadixDNSAliasSpec{ts.dnsAlias.Domain: internal.BuildRadixDNSAlias(appName, ts.dnsAlias.Component, ts.dnsAlias.Environment, ts.dnsAlias.Domain).Spec}),
				"create new or updated RadixDNSAlias")
			handlerSynced := false
			handler := dnsalias.NewHandler(s.KubeClient, s.KubeUtil, s.RadixClient,
				func(synced bool) { handlerSynced = synced }, dnsalias.WithSyncerFactory(s.syncerFactory))
			err = handler.Sync("", ts.dnsAlias.Domain, s.EventRecorder)
			require.NoError(s.T(), err)
			require.True(s.T(), handlerSynced, "Handler should be synced")

		})
	}
}

func registerExistingRadixDNSAliases(radixClient radixclient.Interface, radixDNSAliasesMap map[string]radixv1.RadixDNSAliasSpec) error {
	for domain, rdaSpec := range radixDNSAliasesMap {
		_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.TODO(),
			&radixv1.RadixDNSAlias{
				ObjectMeta: metav1.ObjectMeta{
					Name:   domain,
					Labels: labels.Merge(labels.ForApplicationName(rdaSpec.AppName), labels.ForComponentName(rdaSpec.Component)),
				},
				Spec: rdaSpec,
			}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
