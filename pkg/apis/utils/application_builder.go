package utils

import (
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationBuilder Handles construction of RA
type ApplicationBuilder interface {
	WithRadixRegistration(RegistrationBuilder) ApplicationBuilder
	WithAppName(appName string) ApplicationBuilder
	WithBuildSecrets(buildSecrets ...string) ApplicationBuilder
	WithBuildVariables(buildVariables radixv1.EnvVarsMap) ApplicationBuilder
	WithBuildKit(useBuildKit *bool) ApplicationBuilder
	WithBuildCache(*bool) ApplicationBuilder
	WithEnvironment(environment, buildFrom string) ApplicationBuilder
	WithEnvironmentByBuild(environment string, build radixv1.EnvBuild) ApplicationBuilder
	WithEnvironmentNoBranch(environment string) ApplicationBuilder
	WithComponent(components RadixApplicationComponentBuilder) ApplicationBuilder
	WithComponents(components ...RadixApplicationComponentBuilder) ApplicationBuilder
	WithJobComponent(component RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithJobComponents(components ...RadixApplicationJobComponentBuilder) ApplicationBuilder
	WithDNSAppAlias(env, component string) ApplicationBuilder
	WithDNSAlias(dnsAliases ...radixv1.DNSAlias) ApplicationBuilder
	WithDNSExternalAlias(alias, env, component string, useCertificateAutomation bool) ApplicationBuilder
	WithPrivateImageRegistry(server, username, email string) ApplicationBuilder
	WithApplicationEnvironmentBuilders(environmentBuilders ...ApplicationEnvironmentBuilder) ApplicationBuilder
	WithSubPipeline(subPipelineBuilder SubPipelineBuilder) ApplicationBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRA() *radixv1.RadixApplication
}

// ApplicationBuilderStruct Instance variables
type ApplicationBuilderStruct struct {
	registrationBuilder            RegistrationBuilder
	appName                        string
	buildSecrets                   []string
	useBuildKit                    *bool
	useBuildCache                  *bool
	environments                   []radixv1.Environment
	components                     []RadixApplicationComponentBuilder
	jobComponents                  []RadixApplicationJobComponentBuilder
	dnsAppAlias                    radixv1.AppAlias
	dnsAliases                     []radixv1.DNSAlias
	externalAppAlias               []radixv1.ExternalAlias
	privateImageHubs               radixv1.PrivateImageHubEntries
	applicationEnvironmentBuilders []ApplicationEnvironmentBuilder
	subPipelineBuilder             SubPipelineBuilder
	buildVariables                 radixv1.EnvVarsMap
}

func (ap *ApplicationBuilderStruct) WithBuildKit(useBuildKit *bool) ApplicationBuilder {
	ap.useBuildKit = useBuildKit
	return ap
}
func (ap *ApplicationBuilderStruct) WithBuildCache(useBuildCache *bool) ApplicationBuilder {
	ap.useBuildCache = useBuildCache
	return ap
}

// WithPrivateImageRegistry adds a private image hub to application
func (ap *ApplicationBuilderStruct) WithPrivateImageRegistry(server, username, email string) ApplicationBuilder {
	if ap.privateImageHubs == nil {
		ap.privateImageHubs = map[string]*radixv1.RadixPrivateImageHubCredential{}
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

// WithBuildVariables Sets build variables
func (ap *ApplicationBuilderStruct) WithBuildVariables(buildVariables radixv1.EnvVarsMap) ApplicationBuilder {
	ap.buildVariables = buildVariables
	return ap
}

// WithEnvironment Appends to environment-build list
func (ap *ApplicationBuilderStruct) WithEnvironment(environment, buildFrom string) ApplicationBuilder {
	ap.WithEnvironmentByBuild(environment, radixv1.EnvBuild{
		From: buildFrom,
	})
	return ap
}

// WithEnvironmentByBuild Appends to environment-build list
func (ap *ApplicationBuilderStruct) WithEnvironmentByBuild(environment string, build radixv1.EnvBuild) ApplicationBuilder {
	ap.environments = append(ap.environments, radixv1.Environment{
		Name:  environment,
		Build: build,
	})
	return ap
}

// WithApplicationEnvironmentBuilders Sets app-environment builders
func (ap *ApplicationBuilderStruct) WithApplicationEnvironmentBuilders(environmentBuilders ...ApplicationEnvironmentBuilder) ApplicationBuilder {
	ap.applicationEnvironmentBuilders = environmentBuilders
	return ap
}

// WithSubPipeline sub-pipeline config
func (ap *ApplicationBuilderStruct) WithSubPipeline(subPipelineBuilder SubPipelineBuilder) ApplicationBuilder {
	ap.subPipelineBuilder = subPipelineBuilder
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

// WithDNSAlias Sets alias for env and component to be the DNS alias like "my-alias.radix.equinor.com" or "my-alias.<clustername>.radix.equinor.com"
func (ap *ApplicationBuilderStruct) WithDNSAlias(dnsAliases ...radixv1.DNSAlias) ApplicationBuilder {
	ap.dnsAliases = dnsAliases
	return ap
}

// WithDNSExternalAlias Sets env + component to the external alias
func (ap *ApplicationBuilderStruct) WithDNSExternalAlias(alias, env, component string, useCertificateAutomation bool) ApplicationBuilder {
	if ap.externalAppAlias == nil {
		ap.externalAppAlias = make([]radixv1.ExternalAlias, 0)
	}

	externalAlias := radixv1.ExternalAlias{
		Alias:                    alias,
		Environment:              env,
		Component:                component,
		UseCertificateAutomation: useCertificateAutomation,
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

	radixApplication := &radixv1.RadixApplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: radixv1.SchemeGroupVersion.Identifier(),
			Kind:       radixv1.KindRadixApplication,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ap.appName,
			Namespace: GetAppNamespace(ap.appName),
		},
		Spec: radixv1.RadixApplicationSpec{
			Components:       components,
			Jobs:             jobComponents,
			Environments:     ap.environments,
			DNSAppAlias:      ap.dnsAppAlias,
			DNSAlias:         ap.dnsAliases,
			DNSExternalAlias: ap.externalAppAlias,
			PrivateImageHubs: ap.privateImageHubs,
		},
	}
	if ap.useBuildKit != nil || ap.buildSecrets != nil || len(ap.buildSecrets) > 0 || len(ap.buildVariables) > 0 || ap.subPipelineBuilder != nil {
		radixApplication.Spec.Build = &radixv1.BuildSpec{
			Secrets:       ap.buildSecrets,
			Variables:     ap.buildVariables,
			UseBuildKit:   ap.useBuildKit,
			UseBuildCache: ap.useBuildCache,
		}
		if ap.subPipelineBuilder != nil {
			radixApplication.Spec.Build.SubPipeline = ap.subPipelineBuilder.Build()
		}
	}

	environments := slice.Map(ap.applicationEnvironmentBuilders, func(builder ApplicationEnvironmentBuilder) radixv1.Environment {
		return builder.Build()
	})
	radixApplication.Spec.Environments = append(radixApplication.Spec.Environments, environments...)
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
		WithComponent(AnApplicationComponent().WithPort("http", 8080).WithPublicPort("http")).
		WithJobComponent(
			AnApplicationJobComponent().
				WithSchedulerPort(numbers.Int32Ptr(8888)),
		)

	return builder
}
