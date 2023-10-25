package utils

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/radix"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationBuilder Handles construction of RA
type ApplicationBuilder interface {
	WithRadixRegistration(RegistrationBuilder) ApplicationBuilder
	WithAppName(appName string) ApplicationBuilder
	WithBuildSecrets(buildSecrets ...string) ApplicationBuilder
	WithBuildKit(useBuildKit *bool) ApplicationBuilder
	WithEnvironment(environment, buildFrom string) ApplicationBuilder
	WithEnvironmentNoBranch(environment string) ApplicationBuilder
	WithComponent(components RadixApplicationComponentBuilder) ApplicationBuilder
	WithComponents(components ...RadixApplicationComponentBuilder) ApplicationBuilder
	WithJobComponent(component RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithJobComponents(components ...RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithDNSAppAlias(env, component string) ApplicationBuilder
	WithDNSAlias(dnsAliases ...radixv1.DNSAlias) ApplicationBuilder
	WithDNSExternalAlias(alias, env, component string) ApplicationBuilder
	WithPrivateImageRegistry(server, username, email string) ApplicationBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRA() *radixv1.RadixApplication
}

// ApplicationBuilderStruct Instance variables
type ApplicationBuilderStruct struct {
	registrationBuilder RegistrationBuilder
	appName             string
	buildSecrets        []string
	useBuildKit         *bool
	environments        []radixv1.Environment
	components          []RadixApplicationComponentBuilder
	jobComponents       []RadixApplicationJobComponentBuilder
	dnsAppAlias         radixv1.AppAlias
	dnsAliases          []radixv1.DNSAlias
	externalAppAlias    []radixv1.ExternalAlias
	privateImageHubs    radixv1.PrivateImageHubEntries
}

func (ap *ApplicationBuilderStruct) WithBuildKit(useBuildKit *bool) ApplicationBuilder {
	ap.useBuildKit = useBuildKit
	return ap
}

// WithPrivateImageRegistry adds a private image hub to application
func (ap *ApplicationBuilderStruct) WithPrivateImageRegistry(server, username, email string) ApplicationBuilder {
	if ap.privateImageHubs == nil {
		ap.privateImageHubs = radixv1.PrivateImageHubEntries(map[string]*radixv1.RadixPrivateImageHubCredential{})
	}

	ap.privateImageHubs[server] = &radixv1.RadixPrivateImageHubCredential{
		Username: username,
		Email:    email,
	}
	return ap
}

// WithRadixRegistration Associates this builder with a registration builder
func (ap *ApplicationBuilderStruct) WithRadixRegistration(registrationBuilder RegistrationBuilder) ApplicationBuilder {
	ap.registrationBuilder = registrationBuilder
	return ap
}

// WithAppName Sets app name
func (ap *ApplicationBuilderStruct) WithAppName(appName string) ApplicationBuilder {
	if ap.registrationBuilder != nil {
		ap.registrationBuilder = ap.registrationBuilder.WithName(appName)
	}

	ap.appName = appName
	return ap
}

// WithBuildSecrets Appends to build secrets
func (ap *ApplicationBuilderStruct) WithBuildSecrets(buildSecrets ...string) ApplicationBuilder {
	ap.buildSecrets = buildSecrets
	return ap
}

// WithEnvironment Appends to environment-build list
func (ap *ApplicationBuilderStruct) WithEnvironment(environment, buildFrom string) ApplicationBuilder {
	ap.environments = append(ap.environments, radixv1.Environment{
		Name: environment,
		Build: radixv1.EnvBuild{
			From: buildFrom,
		},
	})

	return ap
}

// WithEnvironmentNoBranch Appends environment with no branch mapping to config
func (ap *ApplicationBuilderStruct) WithEnvironmentNoBranch(environment string) ApplicationBuilder {
	ap.environments = append(ap.environments, radixv1.Environment{
		Name: environment,
	})

	return ap
}

// WithDNSAppAlias Sets env + component to be the app alias like "frontend-myapp-prod.radix.equinor.com" or "frontend-myapp-prod.<clustername>.radix.equinor.com"
func (ap *ApplicationBuilderStruct) WithDNSAppAlias(env, component string) ApplicationBuilder {
	ap.dnsAppAlias = radixv1.AppAlias{
		Environment: env,
		Component:   component,
	}
	return ap
}

// WithDNSAlias Sets domain for env and component to be the DNS alias like "my-domain.radix.equinor.com" or "my-domain.<clustername>.radix.equinor.com"
func (ap *ApplicationBuilderStruct) WithDNSAlias(dnsAliases ...radixv1.DNSAlias) ApplicationBuilder {
	var dnsAliasesToAppend []radixv1.DNSAlias
	for _, dnsAlias := range dnsAliases {
		foundExistingAlias := false
		for i := 0; i < len(ap.dnsAliases); i++ {
			if strings.EqualFold(dnsAlias.Domain, ap.dnsAliases[i].Domain) {
				ap.dnsAliases[i].Domain = dnsAlias.Domain
				ap.dnsAliases[i].Environment = dnsAlias.Environment
				ap.dnsAliases[i].Component = dnsAlias.Component
				foundExistingAlias = true
				break
			}
		}
		if foundExistingAlias {
			continue
		}
		dnsAliasesToAppend = append(dnsAliasesToAppend, radixv1.DNSAlias{
			Domain:      dnsAlias.Domain,
			Environment: dnsAlias.Environment,
			Component:   dnsAlias.Component,
		})
	}
	ap.dnsAliases = append(ap.dnsAliases, dnsAliasesToAppend...)
	return ap
}

// WithDNSExternalAlias Sets env + component to the external alias
func (ap *ApplicationBuilderStruct) WithDNSExternalAlias(alias, env, component string) ApplicationBuilder {
	if ap.externalAppAlias == nil {
		ap.externalAppAlias = make([]radixv1.ExternalAlias, 0)
	}

	externalAlias := radixv1.ExternalAlias{
		Alias:       alias,
		Environment: env,
		Component:   component,
	}

	ap.externalAppAlias = append(ap.externalAppAlias, externalAlias)
	return ap
}

// WithComponent Appends application component to list of existing components
func (ap *ApplicationBuilderStruct) WithComponent(component RadixApplicationComponentBuilder) ApplicationBuilder {
	ap.components = append(ap.components, component)
	return ap
}

// WithComponents Sets application components to application
func (ap *ApplicationBuilderStruct) WithComponents(components ...RadixApplicationComponentBuilder) ApplicationBuilder {
	ap.components = components
	return ap
}

// WithJobComponent Appends application job component to list of existing components
func (ap *ApplicationBuilderStruct) WithJobComponent(component RadixApplicationJobComponentBuilder) ApplicationBuilder {
	ap.jobComponents = append(ap.jobComponents, component)
	return ap
}

// WithJobComponents Sets application job components to application
func (ap *ApplicationBuilderStruct) WithJobComponents(components ...RadixApplicationJobComponentBuilder) ApplicationBuilder {
	ap.jobComponents = components
	return ap
}

// GetRegistrationBuilder Gets associated registration builder
func (ap *ApplicationBuilderStruct) GetRegistrationBuilder() RegistrationBuilder {
	if ap.registrationBuilder != nil {
		return ap.registrationBuilder
	}

	return nil
}

// BuildRA Builds RA
func (ap *ApplicationBuilderStruct) BuildRA() *radixv1.RadixApplication {
	var components = make([]radixv1.RadixComponent, 0)
	for _, comp := range ap.components {
		components = append(components, comp.BuildComponent())
	}

	var jobComponents = make([]radixv1.RadixJobComponent, 0)
	for _, comp := range ap.jobComponents {
		jobComponents = append(jobComponents, comp.BuildJobComponent())
	}
	var build *radixv1.BuildSpec
	if ap.useBuildKit == nil && ap.buildSecrets == nil {
		build = nil
	} else {
		build = &radixv1.BuildSpec{
			Secrets:     ap.buildSecrets,
			UseBuildKit: ap.useBuildKit,
		}
	}

	radixApplication := &radixv1.RadixApplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: radix.APIVersion,
			Kind:       radix.KindRadixApplication,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ap.appName,
			Namespace: GetAppNamespace(ap.appName),
		},
		Spec: radixv1.RadixApplicationSpec{
			Build:            build,
			Components:       components,
			Jobs:             jobComponents,
			Environments:     ap.environments,
			DNSAppAlias:      ap.dnsAppAlias,
			DNSAlias:         ap.dnsAliases,
			DNSExternalAlias: ap.externalAppAlias,
			PrivateImageHubs: ap.privateImageHubs,
		},
	}
	return radixApplication
}

// NewRadixApplicationBuilder Constructor for config builder
func NewRadixApplicationBuilder() ApplicationBuilder {
	return &ApplicationBuilderStruct{}
}

// ARadixApplication Constructor for application builder containing test data
func ARadixApplication() ApplicationBuilder {
	builder := NewRadixApplicationBuilder().
		WithRadixRegistration(ARadixRegistration()).
		WithAppName("anyapp").
		WithEnvironment("test", "master").
		WithComponent(AnApplicationComponent()).
		WithJobComponent(
			AnApplicationJobComponent().
				WithSchedulerPort(numbers.Int32Ptr(8888)),
		)

	return builder
}
