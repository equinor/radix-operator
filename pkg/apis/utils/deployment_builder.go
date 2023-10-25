package utils

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DeploymentBuilder Handles construction of RD
type DeploymentBuilder interface {
	WithRadixApplication(ApplicationBuilder) DeploymentBuilder
	WithRadixDeployment(*v1.RadixDeployment) DeploymentBuilder
	WithDeploymentName(string) DeploymentBuilder
	WithImageTag(string) DeploymentBuilder
	WithAppName(string) DeploymentBuilder
	WithLabel(label, value string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithJobName(string) DeploymentBuilder
	WithCreated(time.Time) DeploymentBuilder
	WithUID(types.UID) DeploymentBuilder
	WithCondition(v1.RadixDeployCondition) DeploymentBuilder
	WithActiveFrom(time.Time) DeploymentBuilder
	WithActiveTo(time.Time) DeploymentBuilder
	WithEmptyStatus() DeploymentBuilder
	WithComponent(DeployComponentBuilder) DeploymentBuilder
	WithComponents(...DeployComponentBuilder) DeploymentBuilder
	WithJobComponent(DeployJobComponentBuilder) DeploymentBuilder
	WithJobComponents(...DeployJobComponentBuilder) DeploymentBuilder
	WithAnnotations(map[string]string) DeploymentBuilder
	GetApplicationBuilder() ApplicationBuilder
	BuildRD() *v1.RadixDeployment
}

// DeploymentBuilderStruct Holds instance variables
type DeploymentBuilderStruct struct {
	annotations        map[string]string
	applicationBuilder ApplicationBuilder
	DeploymentName     string
	AppName            string
	emptyStatus        bool
	Labels             map[string]string
	ImageTag           string
	Environment        string
	Created            time.Time
	Condition          v1.RadixDeployCondition
	ActiveFrom         metav1.Time
	ActiveTo           metav1.Time
	ResourceVersion    string
	UID                types.UID
	components         []DeployComponentBuilder
	jobComponents      []DeployJobComponentBuilder
}

func (db *DeploymentBuilderStruct) WithAnnotations(annotations map[string]string) DeploymentBuilder {
	db.annotations = annotations
	return db
}

// WithDeploymentName Sets name of the deployment
func (db *DeploymentBuilderStruct) WithDeploymentName(name string) DeploymentBuilder {
	db.DeploymentName = name
	return db
}

// WithRadixApplication Links to RA builder
func (db *DeploymentBuilderStruct) WithRadixApplication(applicationBuilder ApplicationBuilder) DeploymentBuilder {
	db.applicationBuilder = applicationBuilder
	return db
}

// WithEmptyStatus Indicates that the RD has no reconciled status
func (db *DeploymentBuilderStruct) WithEmptyStatus() DeploymentBuilder {
	db.emptyStatus = true
	return db
}

// WithRadixDeployment Reverse engineers RD
func (db *DeploymentBuilderStruct) WithRadixDeployment(radixDeployment *v1.RadixDeployment) DeploymentBuilder {
	_, imageTag := GetAppAndTagPairFromName(radixDeployment.Name)

	db.WithImageTag(imageTag)
	db.WithAppName(radixDeployment.Spec.AppName)
	db.WithEnvironment(radixDeployment.Spec.Environment)
	db.WithJobName(radixDeployment.GetLabels()[kube.RadixJobNameLabel])
	db.WithCreated(radixDeployment.CreationTimestamp.Time)
	return db
}

// WithAppName Sets app name
func (db *DeploymentBuilderStruct) WithAppName(appName string) DeploymentBuilder {
	db.Labels["radix-app"] = appName

	if db.applicationBuilder != nil {
		db.applicationBuilder = db.applicationBuilder.WithAppName(appName)
	}

	db.AppName = appName
	return db
}

// WithLabel Appends label
func (db *DeploymentBuilderStruct) WithLabel(label, value string) DeploymentBuilder {
	db.Labels[label] = value
	return db
}

// WithImageTag Sets deployment tag to be appended to name
func (db *DeploymentBuilderStruct) WithImageTag(imageTag string) DeploymentBuilder {
	db.Labels[kube.RadixImageTagLabel] = imageTag
	db.ImageTag = imageTag
	return db
}

// WithEnvironment Sets environment name
func (db *DeploymentBuilderStruct) WithEnvironment(environment string) DeploymentBuilder {
	db.Labels[kube.RadixEnvLabel] = environment
	db.Environment = environment

	if db.applicationBuilder != nil {
		db.applicationBuilder = db.applicationBuilder.WithEnvironmentNoBranch(environment)
	}

	return db
}

// WithJobName Sets RadixJob name
func (db *DeploymentBuilderStruct) WithJobName(name string) DeploymentBuilder {
	db.Labels[kube.RadixJobNameLabel] = name
	return db
}

// WithUID Sets UUID
func (db *DeploymentBuilderStruct) WithUID(uid types.UID) DeploymentBuilder {
	db.UID = uid
	return db
}

// WithCreated Sets timestamp
func (db *DeploymentBuilderStruct) WithCreated(created time.Time) DeploymentBuilder {
	db.Created = created
	return db
}

// WithCondition Sets the condition of the deployment
func (db *DeploymentBuilderStruct) WithCondition(condition v1.RadixDeployCondition) DeploymentBuilder {
	db.Condition = condition
	return db
}

// WithActiveFrom Sets active from
func (db *DeploymentBuilderStruct) WithActiveFrom(activeFrom time.Time) DeploymentBuilder {
	db.ActiveFrom = metav1.NewTime(activeFrom.UTC())
	return db
}

// WithActiveTo Sets active to
func (db *DeploymentBuilderStruct) WithActiveTo(activeTo time.Time) DeploymentBuilder {
	db.ActiveTo = metav1.NewTime(activeTo.UTC())
	return db
}

// WithComponent Appends component to list of components
func (db *DeploymentBuilderStruct) WithComponent(component DeployComponentBuilder) DeploymentBuilder {
	db.components = append(db.components, component)
	return db
}

// WithComponents Sets list of components
func (db *DeploymentBuilderStruct) WithComponents(components ...DeployComponentBuilder) DeploymentBuilder {
	if db.applicationBuilder != nil {
		applicationComponents := make([]RadixApplicationComponentBuilder, 0)

		for _, comp := range components {
			applicationComponents = append(applicationComponents, NewApplicationComponentBuilder().
				WithName(comp.BuildComponent().Name))
		}

		db.applicationBuilder = db.applicationBuilder.WithComponents(applicationComponents...)
	}

	db.components = components
	return db
}

// WithJobComponent Appends job component to list of components
func (db *DeploymentBuilderStruct) WithJobComponent(component DeployJobComponentBuilder) DeploymentBuilder {
	db.jobComponents = append(db.jobComponents, component)
	return db
}

// WithJobComponents Sets list of job components
func (db *DeploymentBuilderStruct) WithJobComponents(components ...DeployJobComponentBuilder) DeploymentBuilder {
	if db.applicationBuilder != nil {
		applicationJobComponents := make([]RadixApplicationJobComponentBuilder, 0)

		for _, comp := range components {
			applicationJobComponents = append(applicationJobComponents, NewApplicationJobComponentBuilder().
				WithName(comp.BuildJobComponent().Name))
		}

		db.applicationBuilder = db.applicationBuilder.WithJobComponents(applicationJobComponents...)
	}

	db.jobComponents = components
	return db
}

// GetApplicationBuilder Obtains the builder for the corresponding RA, if exists (used for testing)
func (db *DeploymentBuilderStruct) GetApplicationBuilder() ApplicationBuilder {
	if db.applicationBuilder != nil {
		return db.applicationBuilder
	}

	return nil
}

// BuildRD Builds RD structure based on set variables
func (db *DeploymentBuilderStruct) BuildRD() *v1.RadixDeployment {
	components := make([]v1.RadixDeployComponent, 0)
	for _, comp := range db.components {
		components = append(components, comp.BuildComponent())
	}
	jobComponents := make([]v1.RadixDeployJobComponent, 0)
	for _, comp := range db.jobComponents {
		jobComponents = append(jobComponents, comp.BuildJobComponent())
	}
	deployName := db.DeploymentName
	if deployName == "" {
		deployName = GetDeploymentName(db.AppName, db.Environment, db.ImageTag)
	}
	status := v1.RadixDeployStatus{}
	if !db.emptyStatus {
		status = v1.RadixDeployStatus{
			Condition:  db.Condition,
			ActiveFrom: db.ActiveFrom,
			ActiveTo:   db.ActiveTo,
		}
	}

	radixDeployment := &v1.RadixDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: radix.APIVersion,
			Kind:       radix.KindRadixDeployment,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              deployName,
			Annotations:       db.annotations,
			Namespace:         GetEnvironmentNamespace(db.AppName, db.Environment),
			Labels:            db.Labels,
			CreationTimestamp: metav1.Time{Time: db.Created},
			ResourceVersion:   db.ResourceVersion,
			UID:               db.UID,
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     db.AppName,
			Components:  components,
			Jobs:        jobComponents,
			Environment: db.Environment,
		},
		Status: status,
	}
	return radixDeployment
}

// NewDeploymentBuilder Constructor for deployment builder
func NewDeploymentBuilder() DeploymentBuilder {
	return &DeploymentBuilderStruct{
		Labels:          make(map[string]string),
		Created:         time.Now().UTC(),
		Condition:       v1.DeploymentActive,
		ActiveFrom:      metav1.NewTime(time.Now().UTC()),
		ResourceVersion: strconv.Itoa(rand.Intn(100)),
	}
}

// ARadixDeployment Constructor for deployment builder containing test data
func ARadixDeployment() DeploymentBuilder {
	replicas := 1
	builder := NewDeploymentBuilder().
		WithRadixApplication(ARadixApplication()).
		WithAppName("someapp").
		WithImageTag("imagetag").
		WithEnvironment("test").
		WithComponent(NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/some-image:imagetag").
			WithName("app").
			WithPort("http", 8080).
			WithPublicPort("http").
			WithReplicas(&replicas)).
		WithJobComponent(NewDeployJobComponentBuilder().
			WithName("job").
			WithImage("radixdev.azurecr.io/job:imagetag").
			WithSchedulerPort(numbers.Int32Ptr(8080)))

	return builder
}
