package applicationconfig_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	dnsaliasconfig "github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
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
		port8080   = 8080
		port9090   = 9090
		domain1    = "domain1"
		domain2    = "domain2"
		domain3    = "domain3"
		domain4    = "domain4"
	)
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
		expectedRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
	}{
		{
			name: "no aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{},
		},
		{
			name: "one alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "aliases for multiple components",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
			},
		},
		{
			name: "aliases for multiple environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
			},
		},
		{
			name: "multiple aliases for one component and environment",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component1},
				).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "aliases for multiple components in different environments",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain2, Environment: env2, Component: component2},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
		},
		{
			name: "one alias, exist other aliases, two expected aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
				WithComponent(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain2: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain2: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "change env and component for existing aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env2, Component: component2}).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
		},
		{
			name: "change port for existing aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port9090},
			},
		},
		{
			name: "swap env and component for existing aliases-s",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env2, Component: component2},
					radixv1.DNSAlias{Domain: domain2, Environment: env1, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env2, Component: component2, Port: port9090},
				domain2: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
		},
		{
			name: "remove single alias",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA)),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{},
		},
		{
			name: "remove multiple aliases, keep other",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1},
					radixv1.DNSAlias{Domain: domain3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
		},
		{
			name: "create new, remove some, change other aliases",
			applicationBuilder: utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).WithEnvironment(env2, branch2).
				WithDNSAlias(
					radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component2},
					radixv1.DNSAlias{Domain: domain3, Environment: env2, Component: component1},
				).
				WithComponents(
					utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA),
					utils.NewApplicationComponentBuilder().WithName(component2).WithPort(portA, port9090).WithPublicPort(portA),
				),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component1, Port: port8080},
				domain2: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
			expectedRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: appName1, Environment: env1, Component: component2, Port: port9090},
				domain3: {AppName: appName1, Environment: env2, Component: component1, Port: port8080},
				domain4: {AppName: appName2, Environment: env1, Component: component1, Port: port9090},
			},
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			tu, kubeClient, kubeUtil, radixClient := setupTest()

			require.NoError(t, commonTest.RegisterRadixDNSAliases(radixClient, ts.existingRadixDNSAliases), "create existing RadixDNSAlias")
			require.NoError(t, applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, ts.applicationBuilder), "register radix application")

			radixDNSAliases, err := radixClient.RadixV1().RadixDNSAliases().List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			if ts.expectedRadixDNSAliases == nil {
				require.Len(t, radixDNSAliases.Items, 0, "not expected Radix DNS aliases")
				return
			}

			require.Len(t, radixDNSAliases.Items, len(ts.expectedRadixDNSAliases), "not matching expected Radix DNS aliases count")
			if len(radixDNSAliases.Items) == len(ts.expectedRadixDNSAliases) {
				for _, radixDNSAlias := range radixDNSAliases.Items {
					if expectedDNSAlias, ok := ts.expectedRadixDNSAliases[radixDNSAlias.Name]; ok {
						assert.Equal(t, expectedDNSAlias.AppName, radixDNSAlias.Spec.AppName, "app name")
						assert.Equal(t, expectedDNSAlias.Environment, radixDNSAlias.Spec.Environment, "environment")
						assert.Equal(t, expectedDNSAlias.Component, radixDNSAlias.Spec.Component, "component")
						assert.Equal(t, expectedDNSAlias.Port, radixDNSAlias.Spec.Port, "port")
						if _, itWasExistingAlias := ts.existingRadixDNSAliases[radixDNSAlias.Name]; !itWasExistingAlias {
							ownerReferences := radixDNSAlias.GetOwnerReferences()
							require.Len(t, ownerReferences, 1)
							ownerReference := ownerReferences[0]
							assert.Equal(t, radixDNSAlias.Spec.AppName, ownerReference.Name, "invalid or empty ownerReference.Name")
							assert.Equal(t, radix.KindRadixApplication, ownerReference.Kind, "invalid or empty ownerReference.Kind")
							assert.NotEmpty(t, ownerReference.UID, "ownerReference.UID is empty")
							require.NotNil(t, ownerReference.Controller, "ownerReference.Controller is nil")
							assert.True(t, *ownerReference.Controller, "ownerReference.Controller is false")
						}
						continue
					}
					assert.Fail(t, fmt.Sprintf("found not expected RadixDNSAlias %s: env %s, component %s, port %d, appName %s",
						radixDNSAlias.GetName(), radixDNSAlias.Spec.Environment, radixDNSAlias.Spec.Component, radixDNSAlias.Spec.Port, radixDNSAlias.Spec.AppName))
				}
			}
		})
	}
}

func Test_DNSAliases_FileOndInvalidAppName(t *testing.T) {
	const (
		appName1   = "any-app1"
		appName2   = "any-app2"
		env1       = "env1"
		component1 = "server1"
		domain1    = "domain1"
		branch1    = "branch1"
		portA      = "port-a"
		port8080   = 8080
	)
	tu, kubeClient, kubeUtil, radixClient := setupTest()

	_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.Background(),
		&radixv1.RadixDNSAlias{
			ObjectMeta: metav1.ObjectMeta{Name: domain1, Labels: map[string]string{kube.RadixAppLabel: appName1}},
			Spec:       radixv1.RadixDNSAliasSpec{AppName: appName2, Environment: env1, Component: component1},
		}, metav1.CreateOptions{})
	require.NoError(t, err, "create existing RadixDNSAlias")

	applicationBuilder := utils.ARadixApplication().WithAppName(appName1).WithEnvironment(env1, branch1).
		WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: env1, Component: component1}).
		WithComponents(utils.NewApplicationComponentBuilder().WithName(component1).WithPort(portA, port8080).WithPublicPort(portA))
	err = applyApplicationWithSync(tu, kubeClient, kubeUtil, radixClient, applicationBuilder)
	require.Error(t, err, "register radix application")
}

func Test_ValidateApplicationCanBeAppliedWithDNSAliases(t *testing.T) {
	const (
		raAppName         = "anyapp"
		otherAppName      = "anyapp2"
		raEnv             = "test"
		raComponentName   = "app"
		raPublicPort      = 8080
		someEnv           = "dev"
		someComponentName = "component-abc"
		somePort          = 9090
		domain1           = "domain1"
		domain2           = "domain2"
	)
	dnsConfig := &dnsaliasconfig.DNSConfig{
		DNSZone:               "dev.radix.equinor.com",
		ReservedAppDNSAliases: dnsaliasconfig.AppReservedDNSAlias{"api": "radix-api"},
		ReservedDNSAlias:      []string{"grafana"},
	}
	var testScenarios = []struct {
		name                    string
		applicationBuilder      utils.ApplicationBuilder
		existingRadixDNSAliases map[string]radixv1.RadixDNSAliasSpec
		expectedValidationError error
	}{
		{
			name:                    "No dns aliases",
			applicationBuilder:      utils.ARadixApplication(),
			expectedValidationError: nil,
		},
		{
			name:                    "Added dns aliases",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
		{
			name:               "Existing dns aliases for the app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: raAppName, Environment: raEnv, Component: raComponentName, Port: raPublicPort},
			},
			expectedValidationError: nil,
		},
		{
			name:               "Existing dns aliases for the app and another app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: raAppName, Environment: raEnv, Component: raComponentName, Port: raPublicPort},
				domain2: {AppName: otherAppName, Environment: someEnv, Component: someComponentName, Port: somePort},
			},
			expectedValidationError: nil,
		},
		{
			name:               "Same domain exists in dns alias for another app",
			applicationBuilder: utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: domain1, Environment: raEnv, Component: raComponentName}),
			existingRadixDNSAliases: map[string]radixv1.RadixDNSAliasSpec{
				domain1: {AppName: otherAppName, Environment: someEnv, Component: someComponentName, Port: somePort},
			},
			expectedValidationError: applicationconfig.RadixDNSAliasAlreadyUsedByAnotherApplicationError(domain1),
		},
		{
			name:                    "Reserved domain api for another app",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: applicationconfig.RadixDNSAliasIsReservedForRadixPlatformApplicationError("api"),
		},
		{
			name:                    "Reserved domain api for another service",
			applicationBuilder:      utils.ARadixApplication().WithDNSAlias(radixv1.DNSAlias{Domain: "grafana", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: applicationconfig.RadixDNSAliasIsReservedForRadixPlatformServiceError("grafana"),
		},
		{
			name:                    "Reserved domain api for this app",
			applicationBuilder:      utils.ARadixApplication().WithAppName("radix-api").WithDNSAlias(radixv1.DNSAlias{Domain: "api", Environment: raEnv, Component: raComponentName}),
			expectedValidationError: nil,
		},
	}

	for _, ts := range testScenarios {
		t.Run(ts.name, func(t *testing.T) {
			_, kubeClient, kubeUtil, radixClient := setupTest()
			err := commonTest.RegisterRadixDNSAliases(radixClient, ts.existingRadixDNSAliases)
			require.NoError(t, err)
			rr := ts.applicationBuilder.GetRegistrationBuilder().BuildRR()
			_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			require.NoError(t, err)
			ra := ts.applicationBuilder.BuildRA()
			applicationConfig := applicationconfig.NewApplicationConfig(kubeClient, kubeUtil, radixClient, rr, ra, dnsConfig)

			actualValidationErr := applicationConfig.ApplyConfigToApplicationNamespace()

			if ts.expectedValidationError == nil {
				require.NoError(t, actualValidationErr)
			} else {
				require.EqualError(t, actualValidationErr, ts.expectedValidationError.Error(), "missing or unexpected error")
			}
		})
	}
}
