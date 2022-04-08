package utils

import (
	"time"

	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// EnvironmentBuilder Handles construction of RE
type EnvironmentBuilder interface {
	WithEnvironmentName(string) EnvironmentBuilder
	WithAppName(string) EnvironmentBuilder
	WithLabel(label, value string) EnvironmentBuilder
	WithAppLabel() EnvironmentBuilder
	WithCreatedTime(time.Time) EnvironmentBuilder
	WithReconciledTime(time.Time) EnvironmentBuilder
	WithUID(types.UID) EnvironmentBuilder
	WithOwner(owner meta.OwnerReference) EnvironmentBuilder
	WithRegistrationOwner(registration *rx.RadixRegistration) EnvironmentBuilder
	WithRegistrationBuilder(builder RegistrationBuilder) EnvironmentBuilder
	WithOrphaned(isOrphan bool) EnvironmentBuilder
	WithEgressConfig(egressRules rx.EgressConfig) EnvironmentBuilder
	WithResourceVersion(version string) EnvironmentBuilder
	GetRegistrationBuilder() RegistrationBuilder
	BuildRE() *rx.RadixEnvironment
}

// EnvironmentBuilderStruct Holds instance variables
type EnvironmentBuilderStruct struct {
	registrationBuilder RegistrationBuilder
	EnvironmentName     string
	AppName             string
	EgressConfig        rx.EgressConfig
	Labels              map[string]string
	AppLabel            bool
	CreatedTime         *time.Time
	ReconciledTime      *time.Time
	Owners              []meta.OwnerReference
	ResourceVersion     string
	IsOrphan            bool
	UID                 types.UID
}

// WithEnvironmentName sets name of the environment
func (eb *EnvironmentBuilderStruct) WithEnvironmentName(name string) EnvironmentBuilder {
	eb.EnvironmentName = name
	return eb
}

// WithAppName sets application name
func (eb *EnvironmentBuilderStruct) WithAppName(appName string) EnvironmentBuilder {
	eb.AppName = appName
	return eb
}

// WithLabel appends label
func (eb *EnvironmentBuilderStruct) WithLabel(label, value string) EnvironmentBuilder {
	eb.Labels[label] = value
	return eb
}

// WithAppLabel appends "[radix-app]=$AppName" label
func (eb *EnvironmentBuilderStruct) WithAppLabel() EnvironmentBuilder {
	eb.AppLabel = true
	return eb
}

// WithUID sets UID
func (eb *EnvironmentBuilderStruct) WithUID(uid types.UID) EnvironmentBuilder {
	eb.UID = uid
	return eb
}

// WithCreatedTime sets created objectmeta timestamp
func (eb *EnvironmentBuilderStruct) WithCreatedTime(created time.Time) EnvironmentBuilder {
	eb.CreatedTime = &created
	return eb
}

// WithReconciledTime sets reconciled status timestamp
func (eb *EnvironmentBuilderStruct) WithReconciledTime(reconciled time.Time) EnvironmentBuilder {
	eb.ReconciledTime = &reconciled
	return eb
}

// WithOwner appends OwnerReference
func (eb *EnvironmentBuilderStruct) WithOwner(owner meta.OwnerReference) EnvironmentBuilder {
	eb.Owners = append(eb.Owners, owner)
	return eb
}

// WithRegistrationOwner appends new OwnerReference to a RadixRegistration
func (eb *EnvironmentBuilderStruct) WithRegistrationOwner(registration *rx.RadixRegistration) EnvironmentBuilder {
	if registration == nil {
		return eb
	}
	trueVar := true
	return eb.WithOwner(meta.OwnerReference{
		APIVersion: "radix.equinor.com/v1",
		Kind:       "RadixRegistration",
		Name:       registration.Name,
		UID:        registration.UID,
		Controller: &trueVar,
	})
}

// WithRegistrationBuilder builds a RadixRegistration and appends new OwnerReference
func (eb *EnvironmentBuilderStruct) WithRegistrationBuilder(builder RegistrationBuilder) EnvironmentBuilder {
	eb.registrationBuilder = builder
	return eb
}

// GetRegistrationBuilder returns its RegistrationBuilder
func (eb *EnvironmentBuilderStruct) GetRegistrationBuilder() RegistrationBuilder {
	return eb.registrationBuilder
}

// WithResourceVersion sets ResourceVersion objectmeta
func (eb *EnvironmentBuilderStruct) WithResourceVersion(version string) EnvironmentBuilder {
	eb.ResourceVersion = version
	return eb
}

// WithOrphaned sets the Orphaned status flag
func (eb *EnvironmentBuilderStruct) WithOrphaned(isOrphan bool) EnvironmentBuilder {
	eb.IsOrphan = isOrphan
	return eb
}

// WithEgressConfig sets the egress configuration for this environment
func (eb *EnvironmentBuilderStruct) WithEgressConfig(egress rx.EgressConfig) EnvironmentBuilder {
	eb.EgressConfig = egress
	return eb
}

// BuildRE builds RE structure based on set variables
func (eb *EnvironmentBuilderStruct) BuildRE() *rx.RadixEnvironment {

	var uniqueName string
	if eb.AppName == "" {
		uniqueName = eb.EnvironmentName
	} else if eb.EnvironmentName == "" {
		uniqueName = eb.AppName
	} else {
		uniqueName = GetEnvironmentNamespace(eb.AppName, eb.EnvironmentName)
	}

	if eb.registrationBuilder != nil {
		eb.WithRegistrationOwner(eb.registrationBuilder.BuildRR())
	}

	radixEnvironment := &rx.RadixEnvironment{
		TypeMeta: meta.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixEnvironment",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:            uniqueName,
			Labels:          eb.Labels,
			ResourceVersion: eb.ResourceVersion,
			UID:             eb.UID,
			OwnerReferences: eb.Owners,
		},
		Spec: rx.RadixEnvironmentSpec{
			AppName: eb.AppName,
			EnvName: eb.EnvironmentName,
			Egress:  eb.EgressConfig,
		},
		Status: rx.RadixEnvironmentStatus{
			Orphaned: eb.IsOrphan,
		},
	}

	if eb.CreatedTime != nil {
		radixEnvironment.ObjectMeta.CreationTimestamp.Time = *eb.CreatedTime
	}
	if eb.ReconciledTime != nil {
		radixEnvironment.Status.Reconciled.Time = *eb.ReconciledTime
	}

	if eb.AppLabel {
		radixEnvironment.ObjectMeta.Labels["radix-app"] = eb.AppName
	}

	return radixEnvironment
}

// NewEnvironmentBuilder constructor for environment builder
func NewEnvironmentBuilder() EnvironmentBuilder {

	return &EnvironmentBuilderStruct{
		Labels:          make(map[string]string),
		CreatedTime:     nil,
		ReconciledTime:  nil,
		ResourceVersion: "",
		Owners:          make([]meta.OwnerReference, 0),
		AppLabel:        false,
		IsOrphan:        true,
	}
}

// ARadixEnvironment constructor for environment builder containing test data
func ARadixEnvironment() EnvironmentBuilder {
	now := time.Now()
	builder := NewEnvironmentBuilder().
		WithAppName("anyapp").
		WithEnvironmentName("anyenv").
		WithResourceVersion("v1.0.0").
		WithAppLabel().
		WithCreatedTime(now).
		WithReconciledTime(now).
		WithRegistrationBuilder(ARadixRegistration()).
		WithUID(uuid.NewUUID())

	return builder
}
