package utils

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationBuilder Handles construction of RA
type ApplicationBuilder interface {
	WithRadixRegistration(RegistrationBuilder) ApplicationBuilder
	WithAppName(string) ApplicationBuilder
	WithEnvironment(string, string) ApplicationBuilder
	WithEnvironmentNoBranch(string) ApplicationBuilder
	WithComponent(RadixApplicationComponentBuilder) ApplicationBuilder
	WithComponents(...RadixApplicationComponentBuilder) ApplicationBuilder
	WithDNSAppAlias(string, string) ApplicationBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRA() *v1.RadixApplication
}

// ApplicationBuilderStruct Instance variables
type ApplicationBuilderStruct struct {
	registrationBuilder RegistrationBuilder
	appName             string
	environments        []v1.Environment
	components          []RadixApplicationComponentBuilder
	dnsAppAlias         v1.AppAlias
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

func (ap *ApplicationBuilderStruct) WithEnvironmentNoBranch(environment string) ApplicationBuilder {
	ap.environments = append(ap.environments, v1.Environment{
		Name: environment,
	})

	return ap
}

func (ap *ApplicationBuilderStruct) WithDNSAppAlias(env string, component string) ApplicationBuilder {
	ap.dnsAppAlias = v1.AppAlias{
		Environment: env,
		Component:   component,
	}
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
			Components:   components,
			Environments: ap.environments,
			DNSAppAlias:  ap.dnsAppAlias,
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

// RadixApplicationComponentBuilder Handles construction of RA component
type RadixApplicationComponentBuilder interface {
	WithName(string) RadixApplicationComponentBuilder
	WithPublic(bool) RadixApplicationComponentBuilder
	WithReplicas(int) RadixApplicationComponentBuilder
	WithPort(string, int32) RadixApplicationComponentBuilder
	WithEnvironmentVariablesMap([]v1.EnvVars) RadixApplicationComponentBuilder
	WithEnvironmentVariable(string, string, string) RadixApplicationComponentBuilder
	WithSecrets(...string) RadixApplicationComponentBuilder
	BuildComponent() v1.RadixComponent
}

type radixApplicationComponentBuilder struct {
	name                 string
	environmentVariables []v1.EnvVars
	public               bool
	replicas             int
	ports                map[string]int32
	secrets              []string
}

func (rcb *radixApplicationComponentBuilder) WithName(name string) RadixApplicationComponentBuilder {
	rcb.name = name
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPublic(public bool) RadixApplicationComponentBuilder {
	rcb.public = public
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithReplicas(replicas int) RadixApplicationComponentBuilder {
	rcb.replicas = replicas
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnvironmentVariablesMap(environmentVariables []v1.EnvVars) RadixApplicationComponentBuilder {
	rcb.environmentVariables = environmentVariables
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnvironmentVariable(environment, name, value string) RadixApplicationComponentBuilder {
	for index, variable := range rcb.environmentVariables {
		if variable.Environment == environment {
			envVariables := rcb.environmentVariables[index].Variables
			envVariables[name] = value
			rcb.environmentVariables[index].Variables = envVariables
			return rcb
		}
	}

	envVariables := make(map[string]string)
	envVariables[name] = value

	rcb.environmentVariables = append(rcb.environmentVariables, v1.EnvVars{
		Environment: environment,
		Variables:   envVariables,
	})

	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithSecrets(secrets ...string) RadixApplicationComponentBuilder {
	rcb.secrets = secrets
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithPort(name string, port int32) RadixApplicationComponentBuilder {
	rcb.ports[name] = port
	return rcb
}

func (rcb *radixApplicationComponentBuilder) BuildComponent() v1.RadixComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range rcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	return v1.RadixComponent{
		Name:                 rcb.name,
		EnvironmentVariables: rcb.environmentVariables,
		Public:               rcb.public,
		Replicas:             rcb.replicas,
		Ports:                componentPorts,
		Secrets:              rcb.secrets,
	}
}

// NewApplicationComponentBuilder Constructor for component builder
func NewApplicationComponentBuilder() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{
		ports:                make(map[string]int32),
		environmentVariables: make([]v1.EnvVars, 0),
	}
}

// AnApplicationComponent Constructor for component builder builder containing test data
func AnApplicationComponent() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{
		name:                 "app",
		ports:                make(map[string]int32),
		environmentVariables: make([]v1.EnvVars, 0),
	}
}
