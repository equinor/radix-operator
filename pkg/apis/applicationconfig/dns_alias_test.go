package applicationconfig_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DNSAliases_CreateUpdateDelete(t *testing.T) {
	const (
		appName1   = "any-app1"
		appName2   = "any-app2"
		env1       = "env1"
		env2       = "env2"
		component1 = "server1"
		component2 = "server2"
		branch1    = "branch1"
		branch2    = "branch2"
		portA      = "port-a"
		portB      = "port-b"
		port8080   = 8080
		port9090   = 9090
		alias1     = "alias1"
		alias2     = "alias2"
		alias3     = "alias3"
		alias4     = "alias4"
	)
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]commonTest.DNSAlias
		expectedRadixDNSAliases map[string]commonTest.DNSAlias
	}{
		{
			name: "no aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{},
		},
		{
			name: "one alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "aliases for multiple components",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Alias: alias2, Environment: env1, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env1, Component: component2},
			},
		},
		{
			name: "aliases for multiple environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Alias: alias2, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env2, Component: component1},
			},
		},
		{
			name: "multiple aliases for one component and environment",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Alias: alias2, Environment: env1, Component: component1},
				).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "aliases for multiple components in different environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Alias: alias2, Environment: env2, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env2, Component: component2},
			},
		},
		{
			name: "one alias, exist other aliases, two expected aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias2: {AppName: appName2, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias2: {AppName: appName2, Environment: env1, Component: component1},
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "change env and component for existing aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env2, Component: component2}).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env2, Component: component2},
			},
		},
		{
			name: "change port for existing aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "alias with a second port as public port2",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).
					WithPorts([]radixv1.ComponentPort{{Name: portA}, {Name: portB}}).WithPublicPort(portB)),
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "swap env and component for existing aliases-s",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env2, Component: component2},
					radixv1.DNSAlias{Alias: alias2, Environment: env1, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env2, Component: component2},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env2, Component: component2},
				alias2: {AppName: appName1, Environment: env1, Component: component1},
			},
		},
		{
			name: "remove single alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{},
		},
		{
			name: "remove multiple aliases, keep other",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Alias: alias3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env1, Component: component2},
				alias3: {AppName: appName1, Environment: env2, Component: component1},
				alias4: {AppName: appName2, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias3: {AppName: appName1, Environment: env2, Component: component1},
				alias4: {AppName: appName2, Environment: env1, Component: component1},
			},
		},
		{
			name: "create new, remove some, change other aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component2},
					radixv1.DNSAlias{Alias: alias3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
				alias2: {AppName: appName1, Environment: env1, Component: component2},
				alias4: {AppName: appName2, Environment: env1, Component: component1},
			},
			expectedRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component2},
				alias3: {AppName: appName1, Environment: env2, Component: component1},
				alias4: {AppName: appName2, Environment: env1, Component: component1},
			},
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			tu, kubeClient, kubeUtil, radixClient := setupTest(t)

			require.NoError(t, commonTest.RegisterRadixDNSAliases(context.Background(), radixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")
			require.NoError(t, applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, ts.applicationBuilder), "register radix application")

			radixDNSAliases, err := radixClient.RadixV1().RadixDNSAliases().List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			if ts.expectedRadixDNSAliases == nil {
				require.Len(t, radixDNSAliases.Items, 0)
				return
			}

			require.Len(t, radixDNSAliases.Items, len(ts.expectedRadixDNSAliases))
			if len(radixDNSAliases.Items) == len(ts.expectedRadixDNSAliases) {
				for _, radixDNSAlias := range radixDNSAliases.Items {
					if expectedDNSAlias, ok := ts.expectedRadixDNSAliases[radixDNSAlias.Name]; ok {
						assert.Equal(t, expectedDNSAlias.AppName, radixDNSAlias.Spec.AppName)
						assert.Equal(t, expectedDNSAlias.Environment, radixDNSAlias.Spec.Environment)
						assert.Equal(t, expectedDNSAlias.Component, radixDNSAlias.Spec.Component)
						if _, itWasExistingAlias := ts.existingRadixDNSAliases[radixDNSAlias.Name]; !itWasExistingAlias {
							ownerReferences := radixDNSAlias.GetOwnerReferences()
							require.Len(t, ownerReferences, 1)
							ownerReference := ownerReferences[0]
							assert.Equal(t, radixDNSAlias.Spec.AppName, ownerReference.Name)
							assert.Equal(t, "radix.equinor.com/v1", ownerReference.APIVersion)
							assert.Equal(t, "RadixRegistration", ownerReference.Kind)
							assert.NotEmpty(t, ownerReference.UID)
							require.NotNil(t, ownerReference.Controller)
							assert.True(t, *ownerReference.Controller)
							require.NotNil(t, ownerReference.BlockOwnerDeletion)
							assert.True(t, *ownerReference.BlockOwnerDeletion)
						}
						continue
					}
					assert.Fail(t, fmt.Sprintf("found not expected RadixDNSAlias %s: env %s, component %s, appName %s",
						radixDNSAlias.GetName(), radixDNSAlias.Spec.Environment, radixDNSAlias.Spec.Component, radixDNSAlias.Spec.AppName))
				}
			}
		})
	}
}

func Test_DNSAliases_ValidDNSAliases(t *testing.T) {
	const (
		appName1   = "any-app1"
		appName2   = "any-app2"
		env1       = "env1"
		component1 = "server1"
		component2 = "server2"
		branch1    = "branch1"
		portA      = "port-a"
		port8080   = 8080
		alias1     = "alias1"
		alias2     = "alias2"
	)
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]commonTest.DNSAlias
		expectedError           error
	}{
		{
			name: "exist aliases with different appName, failed",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName2, Environment: env1, Component: component1},
			},
			expectedError: applicationconfig.ErrDNSAliasUsedByOtherApplication,
		},
		{
			name: "exist aliases with same component and port, no error",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component1},
			},
			expectedError: nil,
		},
		{
			name: "exist different alias with different name, no error",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias2: {AppName: appName2, Environment: env1, Component: component1},
			},
			expectedError: nil,
		},
		{
			name: "exist aliases with different component and port, no error",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]commonTest.DNSAlias{
				alias1: {AppName: appName1, Environment: env1, Component: component2},
			},
			expectedError: nil,
		},
		{
			name: "no existing aliases, no error",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Alias: alias1, Environment: env1, Component: component2}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedError: nil,
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			tu, kubeClient, kubeUtil, radixClient := setupTest(t)
			require.NoError(t, commonTest.RegisterRadixDNSAliases(context.Background(), radixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")
			err := applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, ts.applicationBuilder)
			if ts.expectedError != nil {
				assert.ErrorIs(t, err, ts.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_DNSAlias_ClusterRolesAndBindingCreated(t *testing.T) {
	const (
		appName = "any-app"
		rrUID   = "any-uid"
	)
	tu, kubeClient, kubeUtil, radixClient := setupTest(t)
	rrBuilder := utils.NewRegistrationBuilder().
		WithName(appName).
		WithAdGroups([]string{"admingroup1", "admingroup2"}).
		WithAdUsers([]string{"adminuser1", "adminuser2"}).
		WithReaderAdGroups([]string{"readergroup1", "readergroup2"}).
		WithReaderAdUsers([]string{"readeruser1", "readeruser2"}).
		WithUID(rrUID)
	raBuilder := utils.NewRadixApplicationBuilder().
		WithRadixRegistration(rrBuilder).
		WithAppName(appName).
		WithDNSAlias(
			radixv1.DNSAlias{Alias: "alias1", Environment: "any", Component: "any"},
			radixv1.DNSAlias{Alias: "alias2", Environment: "any", Component: "any"},
		)
	ra, err := tu.ApplyApplication(raBuilder)
	require.NoError(t, err)

	sut := applicationconfig.NewApplicationConfig(kubeClient, kubeUtil, radixClient, rrBuilder.BuildRR(), ra)
	require.NoError(t, sut.OnSync(context.Background()))

	clusterRoles, _ := kubeClient.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{})
	assert.NotEmpty(t, clusterRoles.Items)

	clusterRoleBindings, _ := kubeClient.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	assert.NotEmpty(t, clusterRoleBindings.Items)
}
