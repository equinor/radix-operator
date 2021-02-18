package utils

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationBuilder Handles construction of RA
type ApplicationBuilder interface {
	WithRadixRegistration(RegistrationBuilder) ApplicationBuilder
	WithAppName(string) ApplicationBuilder
	WithBuildSecrets(...string) ApplicationBuilder
	WithEnvironment(string, string) ApplicationBuilder
	WithEnvironmentNoBranch(string) ApplicationBuilder
	WithComponent(RadixApplicationComponentBuilder) ApplicationBuilder
	WithComponents(...RadixApplicationComponentBuilder) ApplicationBuilder
	WithJobComponent(RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithJobComponents(...RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithDNSAppAlias(string, string) ApplicationBuilder
	WithDNSExternalAlias(string, string, string) ApplicationBuilder
	WithPrivateImageRegistry(string, string, string) ApplicationBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRA() *v1.RadixApplication
}

// ApplicationBuilderStruct Instance variables
type ApplicationBuilderStruct struct {
	registrationBuilder RegistrationBuilder
	appName             string
	buildSecrets        []string
	environments        []v1.Environment
	components          []RadixApplicationComponentBuilder
	jobComponents       []RadixApplicationJobComponentBuilder
	dnsAppAlias         v1.AppAlias
	externalAppAlias    []v1.ExternalAlias
	privateImageHubs    v1.PrivateImageHubEntries
}

// WithPrivateImageRegistry adds a private image hub to application
func (ap *ApplicationBuilderStruct) WithPrivateImageRegistry(server, username, email string) ApplicationBuilder {
	if ap.privateImageHubs == nil {
		ap.privateImageHubs = v1.PrivateImageHubEntries(map[string]*v1.RadixPrivateImageHubCredential{})
	}

	ap.privateImageHubs[server] = &v1.RadixPrivateImageHubCredential{
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
	ap.environments = append(ap.environments, v1.Environment{
		Name: environment,
		Build: v1.EnvBuild{
			From: buildFrom,
		},
	})

	return ap
}

// WithEnvironmentNoBranch Appends environment with no branch mapping to config
func (ap *ApplicationBuilderStruct) WithEnvironmentNoBranch(environment string) ApplicationBuilder {
	ap.environments = append(ap.environments, v1.Environment{
		Name: environment,
	})

	return ap
}

// WithDNSAppAlias Sets env + component to be the app alias
func (ap *ApplicationBuilderStruct) WithDNSAppAlias(env string, component string) ApplicationBuilder {
	ap.dnsAppAlias = v1.AppAlias{
		Environment: env,
		Component:   component,
	}
	return ap
}

// WithDNSExternalAlias Sets env + component to the the external alias
func (ap *ApplicationBuilderStruct) WithDNSExternalAlias(alias, env, component string) ApplicationBuilder {
	if ap.externalAppAlias == nil {
		ap.externalAppAlias = make([]v1.ExternalAlias, 0)
	}

	externalAlias := v1.ExternalAlias{
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
func (ap *ApplicationBuilderStruct) BuildRA() *v1.RadixApplication {
	var components = make([]v1.RadixComponent, 0)
	for _, comp := range ap.components {
		components = append(components, comp.BuildComponent())
	}

	var jobComponents = make([]v1.RadixJobComponent, 0)
	for _, comp := range ap.jobComponents {
		jobComponents = append(jobComponents, comp.BuildJobComponent())
	}

	radixApplication := &v1.RadixApplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixApplication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ap.appName,
			Namespace: GetAppNamespace(ap.appName),
		},
		Spec: v1.RadixApplicationSpec{
			Build: &v1.BuildSpec{
				Secrets: ap.buildSecrets,
			},
			Components:       components,
			Jobs:             jobComponents,
			Environments:     ap.environments,
			DNSAppAlias:      ap.dnsAppAlias,
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
		WithComponent(AnApplicationComponent())

	return builder
}
