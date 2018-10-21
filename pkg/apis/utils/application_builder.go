package utils

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationBuilder Handles construction of RA
type ApplicationBuilder interface {
	WithRadixRegistration(RegistrationBuilder) ApplicationBuilder
	WithAppName(string) ApplicationBuilder
	WithEnvironments([]string) ApplicationBuilder
	WithComponent(RadixApplicationComponentBuilder) ApplicationBuilder
	WithComponents([]RadixApplicationComponentBuilder) ApplicationBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRA() *v1.RadixApplication
}

type applicationBuilder struct {
	registrationBuilder RegistrationBuilder
	appName             string
	environments        []string
	components          []RadixApplicationComponentBuilder
}

func (ap *applicationBuilder) WithRadixRegistration(registrationBuilder RegistrationBuilder) ApplicationBuilder {
	ap.registrationBuilder = registrationBuilder
	return ap
}

func (ap *applicationBuilder) WithAppName(appName string) ApplicationBuilder {
	if ap.registrationBuilder != nil {
		ap.registrationBuilder = ap.registrationBuilder.WithName(appName)
	}

	ap.appName = appName
	return ap
}

func (ap *applicationBuilder) WithEnvironments(environments []string) ApplicationBuilder {
	ap.environments = environments
	return ap
}

func (ap *applicationBuilder) WithComponent(component RadixApplicationComponentBuilder) ApplicationBuilder {
	ap.components = append(ap.components, component)
	return ap
}

func (ap *applicationBuilder) WithComponents(components []RadixApplicationComponentBuilder) ApplicationBuilder {
	ap.components = components
	return ap
}

func (ap *applicationBuilder) GetRegistrationBuilder() RegistrationBuilder {
	if ap.registrationBuilder != nil {
		return ap.registrationBuilder
	}

	return nil
}

func (ap *applicationBuilder) BuildRA() *v1.RadixApplication {
	var environments = make([]v1.Environment, 0)
	for _, env := range ap.environments {
		environments = append(environments, v1.Environment{Name: env})
	}

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
			Namespace: getAppNamespace(ap.appName),
		},
		Spec: v1.RadixApplicationSpec{
			Components:   components,
			Environments: environments,
		},
	}
	return radixApplication
}

// NewRadixApplicationBuilder Constructor for config builder
func NewRadixApplicationBuilder() ApplicationBuilder {
	return &applicationBuilder{}
}

// ARadixApplication Constructor for application builder containing test data
func ARadixApplication() ApplicationBuilder {
	builder := NewRadixApplicationBuilder().
		WithRadixRegistration(ARadixRegistration()).
		WithAppName("anyapp").
		WithEnvironments([]string{"test"}).
		WithComponent(NewApplicationComponentBuilder().
			WithName("app"))

	return builder
}

// RadixApplicationComponentBuilder Handles construction of RA component
type RadixApplicationComponentBuilder interface {
	WithName(string) RadixApplicationComponentBuilder
	WithEnvironmentVariablesMap([]v1.EnvVars) RadixApplicationComponentBuilder
	BuildComponent() v1.RadixComponent
}

type radixApplicationComponentBuilder struct {
	name                 string
	environmentVariables []v1.EnvVars
}

func (rcb *radixApplicationComponentBuilder) WithName(name string) RadixApplicationComponentBuilder {
	rcb.name = name
	return rcb
}

func (rcb *radixApplicationComponentBuilder) WithEnvironmentVariablesMap(environmentVariables []v1.EnvVars) RadixApplicationComponentBuilder {
	rcb.environmentVariables = environmentVariables
	return rcb
}

func (rcb *radixApplicationComponentBuilder) BuildComponent() v1.RadixComponent {
	return v1.RadixComponent{
		Name:                 rcb.name,
		EnvironmentVariables: rcb.environmentVariables,
	}
}

// NewApplicationComponentBuilder Constructor for component builder
func NewApplicationComponentBuilder() RadixApplicationComponentBuilder {
	return &radixApplicationComponentBuilder{}
}
